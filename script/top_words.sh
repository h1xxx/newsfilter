#!/bin/sh

export LC_ALL=C

min_len=5
consecutive_words=1
res_limit=48

get_words()
{
	# one of: permalow, blocked, main
	type=${1}

	case ${consecutive_words} in
		1) paste_arg='';;
		2) paste_arg='- -';;
		3) paste_arg='- - -';;
		4) paste_arg='- - - -';;
	esac

	list="$(cut -f7 /home/x/.config/newsfilter/archive.*/hn_${type}.tsv |
		sed 's| |\n|g' |
		grep -Fv -e'(' -e')' -e'[' -e']' -e':' -e'“')"

	list="${list}\n$(cut -f8 /home/x/.config/newsfilter/hn_${type}.tsv |
		sed 's| |\n|g' |
		grep -Fv -e'(' -e')' -e'[' -e']' -e':' -e'“')"

	# don't show phrases with common english words
	list="$(printf "%s" "${list}" |
		paste -d' ' ${paste_arg} |
		grep -v -e'^– ' -e' –$' |
		grep -v -e'^a ' -e' a$' |
		grep -v -e'^A ' -e' A$' |
		grep -v -e'^I ' -e' I$' |
		grep -v -e'^an ' -e' an$' |
		grep -v -e'^at ' -e' at$' |
		grep -v -e'^as ' -e' as$' |
		grep -v -e'^by ' -e' by$' |
		grep -v -e'^be ' -e' be$' |
		grep -v -e'^do ' -e' do$' |
		grep -v -e'^is ' -e' is$' |
		grep -v -e'^in ' -e' in$' |
		grep -v -e'^it ' -e' it$' |
		grep -v -e'^my ' -e' my$' |
		grep -v -e'^no ' -e' no$' |
		grep -v -e'^of ' -e' of$' |
		grep -v -e'^on ' -e' on$' |
		grep -v -e'^so ' -e' so$' |
		grep -v -e'^to ' -e' to$' |
		grep -v -e'^we ' -e' we$' |
		grep -v -e'^If ' -e' If$' |
		grep -v -e'^Is ' -e' Is$' |
		grep -v -e'^It ' -e' It$' |
		grep -v -e'^An ' -e' An$' |
		grep -v -e'^My ' -e' My$' |
		grep -v -e'^and ' -e' and$' |
		grep -v -e'^any ' -e' any$' |
		grep -v -e'^are ' -e' are$' |
		grep -v -e'^can ' -e' can$' |
		grep -v -e'^for ' -e' for$' |
		grep -v -e'^has ' -e' has$' |
		grep -v -e'^how ' -e' how$' |
		grep -v -e'^its ' -e' its$' |
		grep -v -e'^may ' -e' may$' |
		grep -v -e'^not ' -e' not$' |
		grep -v -e'^now ' -e' now$' |
		grep -v -e'^own ' -e' own$' |
		grep -v -e'^say ' -e' say$' |
		grep -v -e'^the ' -e' the$' |
		grep -v -e'^too ' -e' too$' |
		grep -v -e'^who ' -e' who$' |
		grep -v -e'^you ' -e' you$' |
		grep -v -e'^Are ' -e' Are$' |
		grep -v -e'^Ask ' -e' Ask$' |
		grep -v -e'^Has ' -e' Has$' |
		grep -v -e'^How ' -e' How$' |
		grep -v -e'^Its ' -e' Its$' |
		grep -v -e'^The ' -e' The$' |
		grep -v -e'^Who ' -e' Who$' |
		grep -v -e'^Why ' -e' Why$' |
		grep -v -e'^does ' -e' does$' |
		grep -v -e'^from ' -e' from$' |
		grep -v -e'^have ' -e' have$' |
		grep -v -e'^last ' -e' last$' |
		grep -v -e'^like ' -e' like$' |
		grep -v -e'^your ' -e' your$' |
		grep -v -e'^says ' -e' says$' |
		grep -v -e'^than ' -e' than$' |
		grep -v -e'^that ' -e' that$' |
		grep -v -e'^this ' -e' this$' |
		grep -v -e'^what ' -e' what$' |
		grep -v -e'^when ' -e' when$' |
		grep -v -e'^will ' -e' will$' |
		grep -v -e'^with ' -e' with$' |
		grep -v -e'^Have ' -e' Have$' |
		grep -v -e'^Part ' -e' Part$' |
		grep -v -e'^Show ' -e' Show$' |
		grep -v -e'^What ' -e' What$' |
		grep -v -e'^could ' -e' could$' |
		grep -v -e'^worth ' -e' worth$' |
		grep -v -e'^years ' -e' years$' |
		grep -v -e'^Could ' -e' Could$' |
		sort)"
	
	# don't show words already blocked
	tmp_file="/tmp/newsfilter_words_${type}.tmp"
	printf "%s" "${list}" > ${tmp_file}
	list="$(grep -Fvf ~/.config/newsfilter/blocked.keywords ${tmp_file})"
	
	printf "%s" "${list}"
}

print_words()
{
	printf '%s' "${1}" |
		sort |
		awk -v min_len="${min_len}" 'length($0)>=min_len' |
		uniq -c | sort -rn | head -n ${res_limit}
}

sort -uo ~/.config/newsfilter/blocked.keywords \
	~/.config/newsfilter/blocked.keywords

printf '\nhn_permalow\n'
low="$(get_words permalow)"
print_words "${low}"

printf '\nhn_blocked\n'
blocked="$(get_words blocked)"
print_words "${blocked}"

printf '\nhn_main\n'
main="$(get_words main)"
print_words "${main}"

