.PHONY: test race vet check verify

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

check: test race vet

verify:
	docker build -t streamdeck-go-check .
