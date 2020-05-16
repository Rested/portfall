import React, {useEffect, useState} from 'react';
import * as Wails from "@wailsapp/runtime";

const colorMap = {
    "debug": "cyan",
    "info": "grey",
    "warn": "yellow",
    "error": "red",
    "fatal": "darkred"
}

function Console() {
    const [logLines, setLogLines] = useState([])

    useEffect(() => {


        // react to the wails events
        Wails.Events.On("log:debug", msg => {
            console.debug(msg);
            setLogLines(logLines.concat([{level: "debug", message: msg}]))

        });
        Wails.Events.On("log:info", msg => {
            console.info(msg);
            setLogLines(logLines.concat([{level: "info", message: msg}]))

        });
        Wails.Events.On("log:warn", msg => {
            console.warn(msg);
            setLogLines(logLines.concat([{level: "warn", message: msg}]))

        });
        Wails.Events.On("log:error", msg => {
            console.error(msg);
            setLogLines(logLines.concat([{level: "error", message: msg}]))

        });
        Wails.Events.On("log:fatal", msg => {
            console.error(msg);
            setLogLines(logLines.concat([{level: "fatal", message: msg}]))
        });

    }, [])

    return <div style={{backgroundColor: "black", height: "100%"}}>
        {logLines.map(({level, message}) => {
            return <p style={{color: "white"}}><span style={{color: colorMap[level]}}>[{level.toUpperCase()}]</span>: {message}</p>
        })}
    </div>
}

export default Console;