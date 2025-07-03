run:
	go run main.go

build:
	go build -o app

docker-up:
	docker-compose up --build

air:
	air
