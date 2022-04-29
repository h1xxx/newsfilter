#!/bin/sh

nf_dir='/home/x/.local/share/newsfilter'

printf '\nhn_permalow\n'
cut -f7 ${nf_dir}/archive.2021-12-24/hn_permalow.tsv |
	grep -i "${1}" | sort -u
cut -f8 ${nf_dir}/archive.2022-04-29/hn_permalow.tsv |
	grep -i "${1}" | sort -u
cut -f8 ${nf_dir}/hn_permalow.tsv |
	grep -i "${1}" | sort -u

printf '\nhn_blocked\n'
cut -f7 ${nf_dir}/archive.2021-12-24/hn_blocked.tsv |
	grep -i "${1}" | sort -u
cut -f8 ${nf_dir}/archive.2022-04-29/hn_blocked.tsv |
	grep -i "${1}" | sort -u
cut -f8 ${nf_dir}/hn_blocked.tsv |
	grep -i "${1}" | sort -u

printf '\nhn_main\n'
cut -f7 ${nf_dir}/archive.2021-12-24/hn_main.tsv |
	grep -i "${1}" | sort -u
cut -f8 ${nf_dir}/archive.2022-04-29/hn_main.tsv |
	grep -i "${1}" | sort -u
cut -f8 ${nf_dir}/hn_main.tsv |
	grep -i "${1}" | sort -u
