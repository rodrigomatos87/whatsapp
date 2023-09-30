run:
	@go run -race ./cmd/main.go --debug

update:


prod:
	@clear;
	@rm    -rf /tmp/ravi-go;
	@mkdir -p /tmp/ravi-go;
	@cp ./ravi.postman_collection.json /tmp/ravi-go/.;
	@go build -o /tmp/ravi-go/ravi-go cmd/main.go

copy:
	@clear;
	@rm    -rf /tmp/ravi;
	@mkdir -p /tmp/ravi;
	@rsync -uva --exclude mdtest.db ../ravi /tmp/ravi/.;

tests: