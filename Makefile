.PHONY:  dev dev-api dev-extractionservice dev-frontend \
		 setup setup-api setup-extractionservice setup-frontend \
		 build lint test 


dev:
		@make -j3 dev-api dev-extractionservice dev-frontend 

dev-api:
		cd api && go run ./cmd/server

dev-extractionservice:
		cd extraction-service && .venv/bin/uvicorn app.main:app --reload --port 8000

dev-frontend:
		cd frontend && npm run dev 

setup:  setup-api setup-extractionservice setup-frontend 

setup-api:
		cd api && go mod tidy 

setup-extractionservice:
		cd extraction-service && python3 -m venv .venv && \
		.venv/bin/pip install --upgrade pip && \
		.venv/bin/pip install -r requirements.txt

setup-frontend:
		cd frontend && npm install 

build: 
		cd api && go build -o bin/server ./cmd/server 

lint:
		cd api && golangci-lint run ./... 
		cd extraction-service && .venv/bin/ruff check .

