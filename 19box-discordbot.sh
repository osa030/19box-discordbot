#! /usr/bin/bash
# 19box-discordbot start/stop/status script

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

LM=19box-discordbot
BIN=$SCRIPT_DIR/bin
LOG=$SCRIPT_DIR/logs

LOG_FILE=$LOG/$LM.log

# check LM is exists
if [ ! -f $BIN/$LM ]; then
    echo "Error: LM[$LM] not found"
    exit 1
fi

function usage() {
    echo "Usage: $0 [start|stop|status]"
    echo "start: Start the bot"
    echo "stop: Stop the bot"
    echo "status: Show the status of the bot"
    exit 1
}

function check_pid() {
    # use pidof command
    pid=$(pidof $LM)
    echo $pid
}   

function start() {
    pid=$(check_pid)
    if [ -n "$pid" ]; then
        echo "$LM is already running with pid $pid"
    else    
        cd $SCRIPT_DIR
        if [ ! -d $LOG ]; then
            mkdir -p $LOG
        fi
        if [ ! -f $LOG_FILE ]; then
            rm -f $LOG_FILE
        fi
        $BIN/$LM -v --logfile $LOG_FILE > $LOG/$LM.log 2>&1 &
        pid=$!
        echo "Started $LM with pid $pid"
    fi
}

function stop() {
    pid=$(check_pid)
    if [ -n "$pid" ]; then
        kill $pid
        echo "Stopped $LM with pid $pid"
    else
        echo "$LM is not running"
    fi
}

function status() {
    pid=$(check_pid)    
    if [ -n "$pid" ]; then
        echo "Running $LM with pid $pid"
    else
        echo "$LM is not running"
    fi
}

case "$1" in
    start)
        start
        ;;
    stop)
        stop
        ;;
    status)
        status
        ;;
    *)
        usage
        exit 1
        ;;
esac