build:
	go build newsfilter.go
	go build dump-hn.go
	mkdir -p ~/.config/newsfilter
	cp -n blocked.domains blocked.keywords ~/.config/newsfilter/
