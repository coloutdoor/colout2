# Colout2 - Deck Estimator
A simple Go web app to estimate deck building costs, including materials, rails, and height adjustments.

## Features
- Deck cost based on length, width, height, and material.
- Optional rails with material and infill choices.
- Add a customer to an estimate.
- Accept estimate.

## Running It
1. Ensure Go 1.24+ is installed. and CGO is enabled:  
2. `go env CGO_ENABLED`
3. Clone or cd into `~/Github/colout2`.
4. Set up `.env` (optional, defaults to localhost):

```
    SERVER_ADDR=127.0.0.1:8080
```

4. Build and run local dev environment:
```bash
go build -o colout2
./colout2
```
Open `http://localhost:8080` in a browser.

5.  Build and run in a container
```bash
docker build -t colout2:latest .
docker run -p 8080:8080 colout2:latest
```

## Build script
```bash
./build.sh         # Build the Go file
./build.sh docker # Build Docker image colut2:latest
./build.sh deploy # Deploy the Docker image to GCP
```

## DB
 - SQLite
 - This requires CGO, so need to run and build on Linux (wsl)
```bash
sqlite3 estimates.db "SELECT name FROM sqlite_master WHERE type='table';"
sqlite3 estimates.db "SELECT estimate_id, first_name, total_cost, save_date FROM estimates;"
```

## Code
- `main.go`: Web server and flow.
- `deck.go`: Deck cost logic.
- `rails.go`: Rail cost logic.
- `costs.go`: Pricing from costs.yaml.
- `customer.go`: Customer data
- `deck_estimate.go`: 
