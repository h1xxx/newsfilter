#!/bin/sh

get_dist() {
	sum="$(cat /home/x/.config/newsfilter/${1} | wc -l)"
	printf "%s %6s %11s\n" hour count relative

	cut -f2 /home/x/.config/newsfilter/${1} |
		cut -d: -f1 | sort | uniq -c |
		awk -v sum="${sum}" \
			'{printf "%s %8s %10.1f%%\n",$2,$1,$1*100/sum}' |
		sort -n
	printf "%s %5s %10.1f%%\n" total "${sum}" 100
}

printf '\nhn_permalow\n'
get_dist hn_permalow.tsv

printf '\nhn_blocked\n'
get_dist hn_blocked.tsv

printf '\nhn_main\n'
get_dist hn_main.tsv
