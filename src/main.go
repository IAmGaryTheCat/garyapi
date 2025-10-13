package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

const (
	defaultGaryImg   = "Gary76.jpg"
	defaultGooberImg = "goober8.jpg"
	defaultGullyImg  = "Gully1.jpg"
)

var (
	garyImages   []string
	gooberImages []string
	gullyImages  []string
	imageCacheMu sync.RWMutex
)

func cacheFileNames(dirPath string) []string {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		fmt.Printf("Error reading dir %s: %v\n", dirPath, err)
		return nil
	}

	names := make([]string, 0, len(files))
	for _, file := range files {
		if !file.IsDir() {
			names = append(names, file.Name())
		}
	}
	return names
}

func getRandomFileName(images []string, defaultName string) string {
	if len(images) == 0 {
		return defaultName
	}
	return images[rand.Intn(len(images))]
}

func getRandomLineFromFile(filePath string) (string, error) {
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("could not read file %s: %w", filePath, err)
	}

	var lines []string
	err = json.Unmarshal(fileContent, &lines)
	if err != nil {
		return "", fmt.Errorf("could not unmarshal JSON from %s: %w", filePath, err)
	}

	if len(lines) == 0 {
		return "", fmt.Errorf("no lines found in %s", filePath)
	}
	return lines[rand.Intn(len(lines))], nil
}

func extractNumberFromFilename(filename string) int {
	re := regexp.MustCompile(`\d+`)
	match := re.FindString(filename)
	if match == "" {
		return 0
	}
	var number int
	fmt.Sscanf(match, "%d", &number)
	return number
}

func serveRandomImageHandler(images *[]string, defaultImage, imageDir string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		imageCacheMu.RLock()
		imageName := getRandomFileName(*images, defaultImage)
		imageCacheMu.RUnlock()
		c.File(filepath.Join(imageDir, imageName))
	}
}

func serveImageURLHandler(baseURL, imageDir string, images *[]string, defaultImage string) gin.HandlerFunc {
	return func(c *gin.Context) {
		imageCacheMu.RLock()
		imageName := getRandomFileName(*images, defaultImage)
		imageCacheMu.RUnlock()

		number := extractNumberFromFilename(imageName)

		cleanBaseURL := baseURL
		if len(cleanBaseURL) > 0 && cleanBaseURL[len(cleanBaseURL)-1] == '/' {
			cleanBaseURL = cleanBaseURL[:len(cleanBaseURL)-1]
		}
		url := fmt.Sprintf("%s/%s", cleanBaseURL, imageName)

		c.JSON(http.StatusOK, gin.H{
			"url":    url,
			"number": number,
		})
	}
}

func serveRandomLineHandler(filePath string) gin.HandlerFunc {
	return func(c *gin.Context) {
		line, err := getRandomLineFromFile(filePath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		var key string
		switch filepath.Base(filePath) {
		case filepath.Base(os.Getenv("QUOTES_FILE")):
			key = "quote"
		case filepath.Base(os.Getenv("JOKES_FILE")):
			key = "joke"
		default:
			key = "line"
		}

		c.JSON(http.StatusOK, gin.H{key: line})
	}
}

func startDirectoryWatcher(dir string, cache *[]string, label string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Printf("Failed to create watcher for %s: %v\n", label, err)
		return
	}
	err = watcher.Add(dir)
	if err != nil {
		fmt.Printf("Failed to watch directory %s: %v\n", dir, err)
		return
	}

	go func() {
		defer watcher.Close()
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&(fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0 {
					imageCacheMu.Lock()
					*cache = cacheFileNames(dir)
					imageCacheMu.Unlock()
					fmt.Printf("[%s] Cache updated due to event: %s\n", label, event)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				fmt.Printf("[%s] Watcher error: %v\n", label, err)
			}
		}
	}()
}

func main() {
	_ = godotenv.Load()

	runtime.GOMAXPROCS(runtime.NumCPU())
	rand.Seed(time.Now().UnixNano())
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	garyDir := os.Getenv("GARY_DIR")
	gooberDir := os.Getenv("GOOBER_DIR")
	gullyDir := os.Getenv("GULLY_DIR")
	quotesPath := os.Getenv("QUOTES_FILE")
	jokesPath := os.Getenv("JOKES_FILE")

	garyImages = cacheFileNames(garyDir)
	gooberImages = cacheFileNames(gooberDir)
	gullyImages = cacheFileNames(gullyDir)
	startDirectoryWatcher(garyDir, &garyImages, "Gary")
	startDirectoryWatcher(gooberDir, &gooberImages, "Goober")
	startDirectoryWatcher(gullyDir, &gullyImages, "Gully")

	r.Static("/Gary", garyDir)
	r.Static("/Goober", gooberDir)
	r.Static("/Gully", gullyDir)

	imageRoutes := r.Group("/")
	{
		imageRoutes.GET("/gary/image/*path", serveRandomImageHandler(&garyImages, defaultGaryImg, garyDir))
		imageRoutes.GET("/goober/image/*path", serveRandomImageHandler(&gooberImages, defaultGooberImg, gooberDir))
		imageRoutes.GET("/gully/image/*path", serveRandomImageHandler(&gullyImages, defaultGullyImg, gullyDir))
	}

	apiRoutes := r.Group("/")
	{
		garyBaseURL := os.Getenv("GARYURL")
		gooberBaseURL := os.Getenv("GOOBERURL")
		gullyBaseURL := os.Getenv("GULLYURL")

		apiRoutes.GET("/gary", serveImageURLHandler(garyBaseURL, garyDir, &garyImages, defaultGaryImg))
		apiRoutes.GET("/goober", serveImageURLHandler(gooberBaseURL, gooberDir, &gooberImages, defaultGooberImg))
		apiRoutes.GET("/gully", serveImageURLHandler(gullyBaseURL, gullyDir, &gullyImages, defaultGullyImg))
		apiRoutes.GET("/quote", serveRandomLineHandler(quotesPath))
		apiRoutes.GET("/joke", serveRandomLineHandler(jokesPath))

		apiRoutes.GET("/gary/count", func(c *gin.Context) {
			imageCacheMu.RLock()
			count := len(garyImages)
			imageCacheMu.RUnlock()
			c.JSON(http.StatusOK, gin.H{"count": count})
		})
		apiRoutes.GET("/goober/count", func(c *gin.Context) {
			imageCacheMu.RLock()
			count := len(gooberImages)
			imageCacheMu.RUnlock()
			c.JSON(http.StatusOK, gin.H{"count": count})
		})
		apiRoutes.GET("/gully/count", func(c *gin.Context) {
			imageCacheMu.RLock()
			count := len(gullyImages)
			imageCacheMu.RUnlock()
			c.JSON(http.StatusOK, gin.H{"count": count})
		})
	}

	indexFile := os.Getenv("INDEX_FILE")
	if indexFile != "" {
		r.GET("/", func(c *gin.Context) {
			c.Header("Cache-Control", "no-store")
			c.File(indexFile)
		})
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if err := r.Run(":" + port); err != nil {
		fmt.Printf("Failed to start the server: %v\n", err)
	}
}
