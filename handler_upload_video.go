package main

import (
	"context"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const uploadLimit = 1 << 30
	r.Body = http.MaxBytesReader(w, r.Body, uploadLimit)
	videoID, err := uuid.Parse(r.PathValue("videoID"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}
	bearer, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find bearer", err)
		return
	}
	userID, err := auth.ValidateJWT(bearer, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}
	videoMetaData, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Video not found", err)
		return
	}
	if userID != videoMetaData.UserID {
		respondWithError(w, http.StatusUnauthorized, "You can't upload a video for this video", nil)
		return
	}
	file, fileHeader, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't read file", err)
		return
	}
	defer file.Close()
	mediaType, _, err := mime.ParseMediaType(fileHeader.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "malformed mdeia type for request", err)
		return
	}
	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Invalid media type", err)
		return
	}
	tempFile, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create temp file", err)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()
	io.Copy(tempFile, file)
	tempFile.Seek(0, io.SeekStart)
	newTempFilePath, err := processVideoForFastEncoding(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't encode metata for video", err)
		return
	}
	newTempFile, err := os.Open(newTempFilePath)
	defer os.Remove(newTempFile.Name())
	defer newTempFile.Close()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't open file after encoding", err)
		return
	}

	aspectRatio, err := getVideoAspectRatio(newTempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get video aspect ratio", err)
		return
	}

	key := getAssetPath(mediaType)
	finalKey := findFolderForVideoAspectRatio(aspectRatio, key)
	_, err = cfg.s3client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(cfg.s3Bucket),
		Key:         aws.String(finalKey),
		Body:        newTempFile,
		ContentType: aws.String(mediaType),
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error uploading file to S3", err)
		return
	}

	videoURL := cfg.getObjectURL(finalKey)
	video := database.Video{
		ID:           videoID,
		CreatedAt:    videoMetaData.CreatedAt,
		UpdatedAt:    time.Now(),
		ThumbnailURL: videoMetaData.ThumbnailURL,
		VideoURL:     &videoURL,
		CreateVideoParams: database.CreateVideoParams{
			Title:       videoMetaData.Title,
			Description: videoMetaData.Description,
			UserID:      videoMetaData.UserID,
		},
	}
	// url := cfg.getObjectURL(finalKey)
	newVideo, err := cfg.dbVideoToSignedVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get signed URL", err)
		return
	}
	// videoMetaData.VideoURL = &url
	err = cfg.db.UpdateVideo(newVideo)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
		return
	}
	respondWithJSON(w, http.StatusOK, newVideo)

}

func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {
	videoURL := video.VideoURL
	if videoURL == nil {
		return video, nil
	}
	parts := strings.Split(*videoURL, ",")
	if len(parts) != 2 {
		return video, nil
	}
	bucket, key := parts[0], parts[1]
	url, err := generatePresignedURL(cfg.s3client, bucket, key, time.Hour)
	if err != nil {
		return video, err
	}
	video.VideoURL = &url
	return video, nil

}
