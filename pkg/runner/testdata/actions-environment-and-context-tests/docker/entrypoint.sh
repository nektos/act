#!/bin/sh

checkEnvVar() {
	name=$1
	value=$2
	allowEmpty=$3

	if [ -z "$value" ]; then
		echo "Missing environment variable: $name"
		exit 1
	fi

	if [ "$allowEmpty" != "true" ]; then
		test=$(echo "$value" |cut -f 2 -d "=")
		if [ -z "$test" ]; then
			echo "Environment variable is empty: $name"
			exit 1
		fi
	fi

	echo "$value"
}

checkEnvVar "GITHUB_ACTION" "$(env |grep "GITHUB_ACTION=")" false
checkEnvVar "GITHUB_ACTION_REPOSITORY" "$(env |grep "GITHUB_ACTION_REPOSITORY=")" true
checkEnvVar "GITHUB_ACTION_REF" "$(env |grep "GITHUB_ACTION_REF=")" true
