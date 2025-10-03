# Gary API

A minimal HTTP API for serving random images of Gary and Goober, as well as quotes and jokes. Built using Go and Gin, this API is designed to be fast and self-contained.

---

## Endpoints

### Image URLs (JSON)
These endpoints return a JSON object containing a URL to a random image.

- `GET /gary` → `{ "url": "https://...", "number": 1 }`
- `GET /goober` → `{ "url": "https://...", "number": 1 }`
 - `GET /gully` → `{ "url": "https://...", "number": 1 }`

### Raw Images
These endpoints return the image file directly.

- `GET /gary/image` → image/jpeg (or other image type)
- `GET /goober/image` → image/jpeg (or other image type)
 - `GET /gully/image` → image/jpeg (or other image type)

### Quotes and Jokes
Returns a single line from a JSON array.

- `GET /quote` → `{ "quote": "..." }`
- `GET /joke` → `{ "joke": "..." }`

### Counts
These endpoints return the number of images currently available for each category. They are useful for monitoring or UI display.

- `GET /gary/count` → `{ "count": 42 }`
- `GET /goober/count` → `{ "count": 8 }`
- `GET /gully/count` → `{ "count": 10 }`

---

## Environment Variables

Your `.env` file should define the following:

```dotenv
# Port the Go server will run on
PORT=3000

# Public URLs for accessing image resources via CDN or static hosting
GARYURL=https://your-cdn.com/gary/
GOOBERURL=https://your-cdn.com/goober/

# Absolute paths to local image directories (served via /Gary and /Goober routes)
GARY_DIR=/absolute/path/to/public/Gary
GOOBER_DIR=/absolute/path/to/public/Goober

# Absolute paths to JSON files used by /quote and /joke endpoints
QUOTES_FILE=/absolute/path/to/json/quotes.json
JOKES_FILE=/absolute/path/to/json/jokes.json
```

---

## JSON Format

Both `quotes.json` and `jokes.json` should be arrays of strings:

```json
[
  "Success is not for the lazy. –Gary",
  "What kind of music do bubbles hate? Pop."
]
```

---

## Running the Server

```bash
go run main.go
```

Make sure your environment variables and file paths are properly set up before launching.

---

## Contributing

Feel free to open issues or submit pull requests for improvements or bug fixes.
