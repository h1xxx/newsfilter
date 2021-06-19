build:
	go build newsfilter.go
	mkdir -p ~/.config/newsfilter
	cp -n blocked.domains blocked.keywords ~/.config/newsfilter/
