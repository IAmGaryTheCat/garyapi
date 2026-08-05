package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"time"
	"github.com/fsnotify/fsnotify"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"
)

const (
	defaultGaryImg   = "Gary76.jpg"
	defaultGooberImg = "Goober8.jpg"
	defaultGullyImg  = "Gully1.jpg"
	defaultGarfieldImg = "Garfield1.jpg"
	defaultImageCount = 1
	maxImageCount      = 750
)

var (
	garyImages   []string
	gooberImages []string
	gullyImages  []string
	garfieldImages []string
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

func serveRandomImageHandler(images *[]string, defaultImage, imageDir string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set("Cache-Control", "no-store")
		imageCacheMu.RLock()
		imageName := getRandomFileName(*images, defaultImage)
		imageCacheMu.RUnlock()
		return c.SendFile(filepath.Join(imageDir, imageName))
	}
}

func serveImageURLHandler(baseURL string, images *[]string, defaultImage string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		raw := c.Query("count")
		if raw == "" {
			raw = c.Query("n")
		}
		if raw == "" {
			raw = c.Get("X-Image-Count")
		}

		count := defaultImageCount
		if raw != "" {
			var n int
			if _, err := fmt.Sscanf(raw, "%d", &n); err == nil && n >= 1 {
				count = n
				if count > maxImageCount {
					count = maxImageCount
				}
			}
		}

		cleanBaseURL := baseURL
		if len(cleanBaseURL) > 0 && cleanBaseURL[len(cleanBaseURL)-1] == '/' {
			cleanBaseURL = cleanBaseURL[:len(cleanBaseURL)-1]
		}

		imageCacheMu.RLock()
		defer imageCacheMu.RUnlock()

		imgs := *images


		if count == 1 {
			imageName := defaultImage
			if len(imgs) > 0 {
				imageName = imgs[rand.Intn(len(imgs))]
			}
			return c.Status(fiber.StatusOK).JSON(fiber.Map{
				"url":    fmt.Sprintf("%s/%s", cleanBaseURL, imageName),
				"number": extractNumberFromFilename(imageName),
			})
		}


		var chosen []string
		if len(imgs) == 0 {
			chosen = make([]string, count)
			for i := range chosen {
				chosen[i] = defaultImage
			}
		} else {

			n := count
			if n > len(imgs) {
				n = len(imgs)
			}
			perm := rand.Perm(len(imgs))
			chosen = make([]string, n)
			for i := 0; i < n; i++ {
				chosen[i] = imgs[perm[i]]
			}
		}

		results := make([]fiber.Map, 0, len(chosen))
		for _, imageName := range chosen {
			results = append(results, fiber.Map{
				"url":    fmt.Sprintf("%s/%s", cleanBaseURL, imageName),
				"number": extractNumberFromFilename(imageName),
			})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"images": results,
			"count":  len(results),
		})
	}
}

func serveRandomLineHandler(filePath string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		line, err := getRandomLineFromFile(filePath)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
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

		return c.Status(fiber.StatusOK).JSON(fiber.Map{key: line})
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
	startTime := time.Now().UTC()

	runtime.GOMAXPROCS(runtime.NumCPU())
	rand.Seed(time.Now().UnixNano())

	app := fiber.New()
	app.Use(recover.New())
	app.Use(logger.New())

	garyDir := os.Getenv("GARY_DIR")
	gooberDir := os.Getenv("GOOBER_DIR")
	gullyDir := os.Getenv("GULLY_DIR")
	quotesPath := os.Getenv("QUOTES_FILE")
	garfieldDir := os.Getenv("GARFIELD_DIR")
	jokesPath := os.Getenv("JOKES_FILE")

	garyImages = cacheFileNames(garyDir)
	gooberImages = cacheFileNames(gooberDir)
	gullyImages = cacheFileNames(gullyDir)
	garfieldImages = cacheFileNames(garfieldDir)
	startDirectoryWatcher(garyDir, &garyImages, "Gary")
	startDirectoryWatcher(gooberDir, &gooberImages, "Goober")
	startDirectoryWatcher(gullyDir, &gullyImages, "Gully")
	startDirectoryWatcher(garfieldDir, &garfieldImages, "Garfield")

	app.Static("/Gary", garyDir)
	app.Static("/Goober", gooberDir)
	app.Static("/Gully", gullyDir)
	app.Static("/Garfield", garfieldDir)

	app.Get("/gary/image", serveRandomImageHandler(&garyImages, defaultGaryImg, garyDir))
	app.Get("/gary/image/*", serveRandomImageHandler(&garyImages, defaultGaryImg, garyDir))
	app.Get("/goober/image", serveRandomImageHandler(&gooberImages, defaultGooberImg, gooberDir))
	app.Get("/goober/image/*", serveRandomImageHandler(&gooberImages, defaultGooberImg, gooberDir))
	app.Get("/gully/image", serveRandomImageHandler(&gullyImages, defaultGullyImg, gullyDir))
	app.Get("/gully/image/*", serveRandomImageHandler(&gullyImages, defaultGullyImg, gullyDir))
	app.Get("/garfield/image", serveRandomImageHandler(&garfieldImages, defaultGarfieldImg, garfieldDir))
	app.Get("/garfield/image/*", serveRandomImageHandler(&garfieldImages, defaultGarfieldImg, garfieldDir))

	garyBaseURL := os.Getenv("GARYURL")
	gooberBaseURL := os.Getenv("GOOBERURL")
	gullyBaseURL := os.Getenv("GULLYURL")
	garfieldBaseURL := os.Getenv("GARFIELDURL")

	app.Get("/gary", serveImageURLHandler(garyBaseURL, &garyImages, defaultGaryImg))
	app.Get("/goober", serveImageURLHandler(gooberBaseURL, &gooberImages, defaultGooberImg))
	app.Get("/gully", serveImageURLHandler(gullyBaseURL, &gullyImages, defaultGullyImg))
	app.Get("/garfield", serveImageURLHandler(garfieldBaseURL, &garfieldImages, defaultGarfieldImg))
	app.Get("/quote", serveRandomLineHandler(quotesPath))
	app.Get("/joke", serveRandomLineHandler(jokesPath))

	app.Get("/info", func(c *fiber.Ctx) error {
		handlerStart := time.Now()
		c.Set("Cache-Control", "no-store")

		now := time.Now().UTC()
		uptime := now.Sub(startTime)

		resp := fiber.Map{
			"now":           now.Format(time.RFC3339Nano),
			"start_time":    startTime.Format(time.RFC3339Nano),
			"uptime_ms":     uptime.Milliseconds(),
			"go_version":    runtime.Version(),
			"num_goroutine": runtime.NumGoroutine(),
			"num_cpu":       runtime.NumCPU(),
			"gomaxprocs":    runtime.GOMAXPROCS(0),
		}
		resp["latency_ms"] = time.Since(handlerStart).Milliseconds()
		return c.Status(fiber.StatusOK).JSON(resp)
	})

	app.Get("/health", func(c *fiber.Ctx) error {
		c.Set("Cache-Control", "no-store")
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"status": "ok",
		})
	})

	app.Get("/gary/count", func(c *fiber.Ctx) error {
		imageCacheMu.RLock()
		count := len(garyImages)
		imageCacheMu.RUnlock()
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"count": count})
	})
	app.Get("/goober/count", func(c *fiber.Ctx) error {
		imageCacheMu.RLock()
		count := len(gooberImages)
		imageCacheMu.RUnlock()
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"count": count})
	})
	app.Get("/gully/count", func(c *fiber.Ctx) error {
		imageCacheMu.RLock()
		count := len(gullyImages)
		imageCacheMu.RUnlock()
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"count": count})
	})
	app.Get("/garfield/count", func(c *fiber.Ctx) error {
		imageCacheMu.RLock()
		count := len(garfieldImages)
		imageCacheMu.RUnlock()
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"count": count})
	})

	indexFile := os.Getenv("INDEX_FILE")
	if indexFile != "" {
		app.Get("/", func(c *fiber.Ctx) error {
			c.Set("Cache-Control", "no-store")
			return c.SendFile(indexFile)
		})
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if err := app.Listen(":" + port); err != nil {
		fmt.Printf("Failed to start the server: %v\n", err)
	}
}
