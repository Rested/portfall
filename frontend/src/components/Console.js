import React, {useCallback, useEffect, useState} from 'react';
import * as Wails from "@wailsapp/runtime";
import IconButton from "@material-ui/core/IconButton";
import {GitHub} from "@material-ui/icons";
import Button from "@material-ui/core/Button";

const colorMap = {
    "debug": "cyan",
    "info": "grey",
    "warn": "yellow",
    "error": "red",
    "fatal": "darkred"
}

function Console() {
    const [logLines, setLogLines] = useState([])

    const handleMessage = useCallback(msg => {
        setLogLines(prevLines => prevLines.concat([msg]))
    }, [])

    useEffect(() => {


        // react to the wails events
        Wails.Events.On("log:debug", msg => {
            console.debug(msg);
            handleMessage({level: "debug", message: msg})

        });
        Wails.Events.On("log:info", msg => {
            console.info(msg);
            handleMessage({level: "info", message: msg})

        });
        Wails.Events.On("log:warn", msg => {
            console.warn(msg);
            handleMessage({level: "warn", message: msg})

        });
        Wails.Events.On("log:error", msg => {
            console.error(msg);
            handleMessage({level: "error", message: msg})

        });
        Wails.Events.On("log:fatal", msg => {
            console.error(msg);
            handleMessage({level: "fatal", message: msg})
        });

    }, [handleMessage])

    return <React.Fragment>
        <Button style={{marginBottom: "1em"}} variant="outlined" onClick={() => {
            const title = encodeURI("Issue with Portfall")
            const body = encodeURI("My problem is ...\nI received the following console output:\n\n```\n" +
                logLines.map(({level, message}) => `[${level.toUpperCase()}]: ${message}`).join("\n") + "\n```")
            window.backend.PortfallOS.OpenInBrowser(`https://github.com/rekon-oss/portfall/issues/new?title=${title}&body=${body}`)
        }}><span style={{marginRight: "1em"}}>Open Issue</span><GitHub/>
        </Button>
        <div style={{backgroundColor: "black", height: "100%"}}>
            {logLines.map(({level, message}) => {
                return <p style={{color: "white", padding: 0, margin: 0}}><span
                    style={{color: colorMap[level]}}>[{level.toUpperCase()}]</span>: {message}</p>
            })}
        </div>
    </React.Fragment>
}

export default Console;