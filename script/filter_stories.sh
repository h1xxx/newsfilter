#!/bin/sh

printf '\nhn_permalow\n'
cut -f7 /home/x/.config/newsfilter/archive.2021-12-24/hn_permalow.tsv |
	grep -i "${1}" | sort -u
cut -f8 /home/x/.config/newsfilter/hn_permalow.tsv |
	grep -i "${1}" | sort -u

printf '\nhn_blocked\n'
cut -f7 /home/x/.config/newsfilter/archive.2021-12-24/hn_blocked.tsv |
	grep -i "${1}" | sort -u
cut -f8 /home/x/.config/newsfilter/hn_blocked.tsv |
	grep -i "${1}" | sort -u

printf '\nhn_main\n'
cut -f7 /home/x/.config/newsfilter/archive.2021-12-24/hn_main.tsv |
	grep -i "${1}" | sort -u
cut -f8 /home/x/.config/newsfilter/hn_main.tsv |
	grep -i "${1}" | sort -u
