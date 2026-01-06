#! /usr/bin/bash


if [ ! -d "bin" ]; then
	mkdir -p bin
else
    rm bin/19box-discordbot 
fi

go build -o bin/19box-discordbot ./cmd/discordbot