build:
	go build newsfilter.go
	go build dump-hn.go

install:
	mkdir -p ~/.local/share/newsfilter
	cp blocked.domains blocked.keywords ~/.local/share/newsfilter/

bin-install:
	mkdir -p $(DESTDIR)/bin
	install -o root -g root -m755 newsfilter $(DESTDIR)/bin/ || :

