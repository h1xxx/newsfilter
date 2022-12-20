build:
	go build newsfilter.go
	go build dump-hn.go

install:
	mkdir -p ~/.local/share/newsfilter
	cp blocked.domains blocked.keywords ~/.local/share/newsfilter/

bin-install:
	mkdir -p $(DESTDIR)/bin
	install -o root -g root -m755 newsfilter $(DESTDIR)/bin/ || :

sort:
	LC_ALL=en_US.utf8 sort -uo blocked.domains blocked.domains
	LC_ALL=en_US.utf8 sort -uo blocked.keywords blocked.keywords

