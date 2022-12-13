build:
	go build newsfilter.go
	go build dump-hn.go

install:
	mkdir -p ~/.local/share/newsfilter
	cp blocked.domains blocked.keywords ~/.local/share/newsfilter/
