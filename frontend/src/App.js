import React, {useEffect, useRef, useState} from 'react';
import FormControl from "@material-ui/core/FormControl";
import {makeStyles} from "@material-ui/core/styles";
import AppBar from "@material-ui/core/AppBar";
import Toolbar from "@material-ui/core/Toolbar";
import Typography from "@material-ui/core/Typography";
import Autocomplete from '@material-ui/lab/Autocomplete';
import TextField from "@material-ui/core/TextField";
import Grid from "@material-ui/core/Grid";
import {Close, Folder, Launch, MoodBadTwoTone, Settings} from "@material-ui/icons";
import Alert from "@material-ui/lab/Alert";
import {Card, CircularProgress} from "@material-ui/core";
import Avatar from "@material-ui/core/Avatar";
import CardHeader from "@material-ui/core/CardHeader";
import Button from "@material-ui/core/Button";
import createMuiTheme from "@material-ui/core/styles/createMuiTheme";
import {ThemeProvider} from "@material-ui/styles";
import Popper from "@material-ui/core/Popper";
import CardContent from "@material-ui/core/CardContent";
import IconButton from "@material-ui/core/IconButton";
import Select from "@material-ui/core/Select";
import MenuItem from "@material-ui/core/MenuItem";
import InputLabel from "@material-ui/core/InputLabel";

const useStyles = makeStyles(theme => ({
    formControl: {
        margin: theme.spacing(1),
        color: '#fff',
        flexGrow: 1,
    },
    selectEmpty: {
        marginTop: theme.spacing(2),
    },
    title: {
        flexGrow: 1
    },
    cardHeaderTitle: {
        minWidth: 10
    },
    inputRoot: {
        color: "white",
        "& .MuiOutlinedInput-notchedOutline": {
            borderColor: "white"
        },
        "&:hover .MuiOutlinedInput-notchedOutline": {
            borderColor: "white"
        },
        "&.Mui-focused .MuiOutlinedInput-notchedOutline": {
            borderColor: "white"
        },

    },
    endAdornment: {
        "& .MuiButtonBase-root": {
            color: "white"
        }
    },
    inputLabelRoot: {
        color: "white"
    }
}));
const theme = createMuiTheme({
    palette: {
        primary: {main: "rgb(50,109,230)"},
        secondary: {main: "#ccc"}
    },
});

function usePrevious(value) {
    const ref = useRef();
    useEffect(() => {
        ref.current = value;
    });
    return ref.current;
}

function App() {
    const classes = useStyles();
    const [namespaces, setNamespaces] = useState([]);
    const [selectedNamespaces, setSelectedNS] = useState([]);
    const [anchorEl, setAnchorEl] = useState(null);
    const prevSelectedNamespaces = usePrevious(selectedNamespaces);
    const [configFilePath, setConfigFilePath] = useState(null)
    const confPathEl = useRef(null)
    const [loading, setLoading] = useState(true);
    const [websites, setWebsites] = useState([]);
    const [showConfig, setShowConfig] = useState(false);
    const [configMessage, setConfigMessage] = useState(null);
    const [availableContexts, setAvailableContexts] = useState([]);
    const [currentContext, setCurrentContext] = useState(null);
    const prevContext = usePrevious(currentContext);

    useEffect(() => {
        window.backend.Client.GetCurrentConfigPath().then(cp => {
            if (cp) {
                setConfigFilePath(cp);
                refreshContext();
            }
        });

    }, []);

    const refreshContext = () => {
        setWebsites([]);
        setLoading(true);
        window.backend.Client.ListNamespaces().then((r) => {
            setNamespaces(r);
            setSelectedNS(["default"]);
            setLoading(false)
        });
        Promise.all([window.backend.Client.GetAvailableContexts(), window.backend.Client.GetCurrentContext()]).then(([acs, cc]) => {
            console.log("current context", cc, ", available:", acs);
            setAvailableContexts(acs);
            setCurrentContext(cc);
        });

    }

    useEffect(() => {
        if (selectedNamespaces !== null && prevSelectedNamespaces !== selectedNamespaces) {
            setLoading(true);
            // get ns to add
            const nsToAdd = selectedNamespaces.find(ns => !(prevSelectedNamespaces || []).includes(ns));
            const namespacesToRemove = (prevSelectedNamespaces || []).filter(ns => !selectedNamespaces.includes(ns));
            let newWebsites = websites;
            if (namespacesToRemove) {
                Promise.all(namespacesToRemove.map(ns => window.backend.Client.RemoveWebsitesInNamespace(ns))).then(() => {
                    console.log("removed namespaces", namespacesToRemove)
                });
                newWebsites = websites.filter(w => {
                    if (namespacesToRemove.includes(w.namespace)) {
                        return false
                    }
                    if (namespacesToRemove.includes("All Namespaces") && !selectedNamespaces.includes(w.namespace)) {
                        return false
                    }
                    return true
                });
            }
            if (nsToAdd) {
                console.log("adding ns", nsToAdd)
                window.backend.Client.GetWebsitesInNamespace(nsToAdd).then(results => {
                    console.log("received websites in ns to add", results)
                    // when we already have all namespaces there is no need to concat the results of the given website
                    if (results && !(prevSelectedNamespaces || []).includes("All Namespaces")) {
                        const resObj = JSON.parse(results)
                        if (resObj){
                            newWebsites = newWebsites.concat(resObj);
                        }
                    }
                    // if we just added All namespaces the portforwards will be reset so we sh
                    setWebsites(newWebsites);
                    setLoading(false);
                });
            } else {
                setWebsites(newWebsites);
                setLoading(false)
            }

        }
    }, [selectedNamespaces, prevSelectedNamespaces, websites]);


    return (
        <ThemeProvider theme={theme}>
            <img src="/blueicon.png" style={{
                position: "absolute",
                top: 0,
                bottom: 0,
                left: 0,
                right: 0,
                margin: "auto",
                width: "60%",
                opacity: 0.01
            }} alt="Blue Portfall logo in background"/>
            <div id="app" className="App">
                <AppBar id="Controls">
                    <Toolbar>
                        <img alt="White Portfall logo in toolbar" src="/whiteicon.png" style={{width: 40, marginRight: "1em"}}/>
                        <Typography className={classes.title} variant="h6" noWrap>
                            Portfall
                        </Typography>
                        {/* todo: arrange by namespace */}
                        <FormControl className={classes.formControl}>
                            <Autocomplete options={["All Namespaces"].concat(namespaces)}
                                          multiple
                                          value={selectedNamespaces}
                                          classes={{
                                              inputRoot: classes.inputRoot,
                                              endAdornment: classes.endAdornment
                                          }}
                                          onChange={(_, value) => {
                                              //
                                              setSelectedNS(value)
                                          }}
                                          renderInput={params => (
                                              <TextField {...params} style={{color: 'white'}}
                                                         label="Namespaces"
                                                         InputLabelProps={{
                                                             color: "secondary",
                                                             classes: {root: classes.inputLabelRoot}
                                                         }}
                                                         variant="outlined"/>)
                                          }/>
                        </FormControl>
                    </Toolbar>
                </AppBar>
                <div id="Websites" style={{
                    paddingTop: 100,
                    flex: 1,
                    alignItems: 'center',
                    justifyContent: 'center',
                    display: 'flex'
                }}>
                    <Grid container spacing={3} style={{
                        flexGrow: 1,
                        padding: "0 2rem"
                    }}>
                        {!configFilePath ? (<Grid item xs={12}>
                            <Alert icon={<MoodBadTwoTone/>} severity="error">
                                <Typography>No config file found</Typography>
                            </Alert>
                        </Grid>) : null}
                        {(configFilePath && websites.length === 0) ? (<Grid item xs={12}>
                            <Alert icon={<MoodBadTwoTone/>} severity="info">
                                <Typography>No websites found to port-forward in the selected namespace(s)</Typography>
                            </Alert>
                        </Grid>) : null}
                        {websites.map(({localPort, podPort, title, iconRemoteUrl}) => (
                            <Grid item xs={4} key={localPort}>
                                <Card>
                                    <CardHeader classes={{content: classes.cardHeaderTitle}}
                                                avatar={<Avatar src={iconRemoteUrl}/>}
                                                title={<Typography noWrap>{title}</Typography>}
                                                subheader={<span><b>{localPort}</b>:{podPort}</span>} action={
                                        <Button endIcon={<Launch/>} size="small" color="primary"
                                                onClick={() =>
                                                    window.backend.PortfallOS.OpenInBrowser(`http://localhost:${localPort}`)}>
                                            Open
                                        </Button>}/>

                                </Card>
                            </Grid>
                        ))}
                        {loading ? <Grid item xs={12} style={{textAlign: 'center'}}><CircularProgress/></Grid> : null}
                    </Grid>
                </div>
                <IconButton variant="contained" color="primary" style={{position: 'fixed', bottom: 10, right: 10}}
                            size="medium"
                            onClick={(e) => {
                                setAnchorEl(e.currentTarget);
                                setShowConfig(!showConfig)
                            }}
                >
                    <Settings/>
                </IconButton>


                <Popper open={showConfig} anchorEl={anchorEl} placement="top-end"
                        onClose={() => setShowConfig(false)} disablePortal={false}
                        modifiers={{
                            flip: {
                                enabled: true,
                            },
                            arrow: {
                                enabled: false,
                                // element: arrowRef,
                            },
                        }}>
                    <Card square style={{width: "50vw"}}>
                        <CardHeader title="Portfall config" action={<IconButton style={{verticalAlign: 'middle'}}
                                                                                onClick={() => setShowConfig(false)}><Close/></IconButton>}/>
                        <CardContent>

                            <Grid container spacing={3}>
                                <Grid item xs={8}>
                                    <TextField defaultValue={configFilePath} fullWidth
                                               disabled={false} label="Kubernetes config file" inputRef={confPathEl}/>
                                </Grid>
                                <Grid item xs={4} style={{alignSelf: "flex-end", textAlign: "end"}}>
                                    <Button variant="contained" color="secondary" endIcon={<Folder/>}
                                            onClick={() => {
                                                window.backend.PortfallOS.OpenFile().then(f => {
                                                    if (f) {
                                                        confPathEl.current.value = f;
                                                    }
                                                })
                                            }}>Browse</Button>
                                </Grid>
                                { (availableContexts && currentContext) ?
                                <Grid item xs={8}>
                                    <FormControl>
                                        <InputLabel>Config context</InputLabel>
                                        <Select value={currentContext} onChange={({target:{value}}) => setCurrentContext(value)}>
                                            {availableContexts.map(c => <MenuItem value={c}>{c}</MenuItem>)}
                                        </Select>
                                    </FormControl>
                                </Grid> : null}
                                {configMessage ? (
                                    <Grid item xs={12}>
                                        <Alert severity={configMessage.severity} onClose={() => {
                                            setConfigMessage(null)
                                        }}>
                                            {configMessage.message}
                                        </Alert>
                                    </Grid>) : null}
                                <Grid item xs={12} style={{textAlign: "center"}}>
                                    <Button color="primary" variant="outlined" onClick={() => {
                                        window.backend.Client.SetConfigPath(confPathEl.current.value, currentContext).then(([rv, ctx]) => {
                                            if (rv === confPathEl.current.value) {
                                                setConfigFilePath(rv);
                                                refreshContext();
                                                setConfigMessage({
                                                    severity: "success",
                                                    message: `Successfully set config to path ${rv} and context to ${ctx}`
                                                })
                                            } else {
                                                // reset context
                                                setCurrentContext(ctx);
                                                setConfigMessage({
                                                    severity: "error",
                                                    message: `Failed to change config to path ${confPathEl.current.value}`
                                                });
                                                confPathEl.current.value = rv;
                                            }
                                        })
                                    }}>Update Config</Button>
                                </Grid>
                            </Grid>

                        </CardContent>

                    </Card>

                </Popper>

            </div>
        </ThemeProvider>
    );
}

export default App;
