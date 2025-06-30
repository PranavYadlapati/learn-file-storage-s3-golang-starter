package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"slices"
	"strings"

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
	mediaType, _, err := mime.ParseMediaType(fileHeader.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unknown media type", err)
		return
	}
	if !slices.Contains([]string{"image/jpeg", "image/png"}, mediaType) {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Invalid media type: %v", mediaType), nil)
		return
	}

	videoMetaData, err := cfg.db.GetVideo(videoID)

	if err != nil {
		respondWithError(w, http.StatusNotFound, "Video not found", err)
		return
	}

	if userID != videoMetaData.UserID {
		respondWithError(w, http.StatusUnauthorized, "You can't upload a thumbnail for this video", nil)
		return
	}
	fileExtension := strings.Split(mediaType, "/")[1]
	filePath := strings.Join([]string{cfg.assetsRoot, fmt.Sprintf("%s.%s", videoID, fileExtension)}, "/")
	new_file, err := os.Create(filePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create file", err)
		return
	}
	io.Copy(new_file, file)
	newURL := fmt.Sprintf("http://localhost:%s/assets/%s.%s", cfg.port, videoID, fileExtension)
	videoMetaData.ThumbnailURL = &newURL
	err = cfg.db.UpdateVideo(videoMetaData)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
		return
	}
	respondWithJSON(w, http.StatusOK, videoMetaData)
	// // respondWithJSON(w, http.StatusOK, struct{}{})
}
