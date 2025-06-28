package main

import (
	"fmt"
	"io"
	"net/http"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
	// _ "github.com/mattn/go-sqlite3"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)
	// TODO: implement the upload here
	const maxMemory = 10 << 20
	r.ParseMultipartForm(maxMemory)
	file, fileHeader, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't read file", err)
		return
	}
	defer file.Close()
	mediaType := fileHeader.Header.Get("Content-Type")
	videoMetaData, err := cfg.db.GetVideo(videoID)

	if err != nil {
		respondWithError(w, http.StatusNotFound, "Video not found", err)
		return
	}

	if userID != videoMetaData.UserID {
		respondWithError(w, http.StatusUnauthorized, "You can't upload a thumbnail for this video", nil)
		return
	}
	data, err := io.ReadAll(file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't read file", err)
		return
	}
	tn := thumbnail{data: data, mediaType: mediaType}
	videoThumbnails[videoID] = tn
	tnUrl := fmt.Sprintf("http://localhost:%s/api/thumbnails/%s", cfg.port, videoID)
	videoMetaData.ThumbnailURL = &tnUrl
	err = cfg.db.UpdateVideo(videoMetaData)
	if err != nil {
		delete(videoThumbnails, videoID)
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, videoMetaData)
	// // respondWithJSON(w, http.StatusOK, struct{}{})
}
