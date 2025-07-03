package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Faster encoding for MP4 files by moving all metadata to the start (usually is at end)
func processVideoForFastEncoding(filePath string) (string, error) {
	newFile := filePath + ".processing"
	command := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", newFile) // Moves metadata to the front
	err := command.Run()
	if err != nil {
		return "", err
	}
	return newFile, nil
}

func (cfg apiConfig) ensureAssetsDir() error {
	if _, err := os.Stat(cfg.assetsRoot); os.IsNotExist(err) {
		return os.Mkdir(cfg.assetsRoot, 0755)
	}
	return nil
}

func (cfg apiConfig) getObjectURL(key string) string {
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, key)
}

func findFolderForVideoAspectRatio(aspectRatio string, fileName string) string {
	switch aspectRatio {
	case "16:9":
		return fmt.Sprintf("landscape/%s", fileName)
	case "9:16":
		return fmt.Sprintf("portrait/%s", fileName)
	default:
		return fmt.Sprintf("other/%s", fileName)
	}
}

func getAssetPath(mediaType string) string {
	base := make([]byte, 32)
	_, err := rand.Read(base)
	if err != nil {
		panic("failed to generate random bytes")
	}
	id := base64.RawURLEncoding.EncodeToString(base)

	ext := mediaTypeToExt(mediaType)
	return fmt.Sprintf("%s%s", id, ext)
}

func (cfg apiConfig) getAssetDiskPath(assetPath string) string {
	return filepath.Join(cfg.assetsRoot, assetPath)
}

func (cfg apiConfig) getAssetURL(assetPath string) string {
	return fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, assetPath)
}

func mediaTypeToExt(mediaType string) string {
	parts := strings.Split(mediaType, "/")
	if len(parts) != 2 {
		return ".bin"
	}
	return "." + parts[1]
}

func approx(a, b float64) bool {
	return math.Abs(a-b) <= float64(0.01)
}

func getVideoAspectRatio(filePath string) (string, error) {
	var output bytes.Buffer
	command := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
	command.Stdout = &output
	command.Run()
	type dimensions struct {
		Streams []struct {
			Width  float64 `json:"width"`
			Height float64 `json:"height"`
		} `json:"streams"`
	}

	var videoDimensions dimensions
	err := json.Unmarshal(output.Bytes(), &videoDimensions)
	if err != nil {
		return "", err
	}
	aspect_ratio := videoDimensions.Streams[0].Width / videoDimensions.Streams[0].Height
	if approx(float64(16)/float64(9), aspect_ratio) {
		return "16:9", nil
	} else if approx(float64(9)/float64(16), aspect_ratio) {
		return "9:16", nil
	} else {
		return "other", nil
	}

}
