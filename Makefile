build:
	go build newsfilter.go
	go build dump-hn.go
	mkdir -p ~/.local/share/newsfilter
	cp -n blocked.domains blocked.keywords ~/.local/share/newsfilter/
