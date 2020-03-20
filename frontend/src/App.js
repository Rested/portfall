import React, {useEffect, useState} from 'react';
import FormControl from "@material-ui/core/FormControl";
import {makeStyles} from "@material-ui/core/styles";
import AppBar from "@material-ui/core/AppBar";
import Toolbar from "@material-ui/core/Toolbar";
import Typography from "@material-ui/core/Typography";
import Autocomplete from '@material-ui/lab/Autocomplete';
import TextField from "@material-ui/core/TextField";
import CircularProgress from "@material-ui/core/CircularProgress";
import Grid from "@material-ui/core/Grid";
import Paper from "@material-ui/core/Paper";
import Chip from "@material-ui/core/Chip";
import {MoodBadTwoTone} from "@material-ui/icons";
import Alert from "@material-ui/lab/Alert";
import {Card} from "@material-ui/core";
import Avatar from "@material-ui/core/Avatar";
import CardHeader from "@material-ui/core/CardHeader";
import CardContent from "@material-ui/core/CardContent";
import Button from "@material-ui/core/Button";
import CardActions from "@material-ui/core/CardActions";
const useStyles = makeStyles(theme => ({
    formControl: {
        margin: theme.spacing(1),
        minWidth: 120,
        color: '#fff'
    },
    selectEmpty: {
        marginTop: theme.spacing(2),
    },
    title: {
        flexGrow: 1
    }
}));

function App() {
    const classes = useStyles();
    const [namespaces, setNamespaces] = useState([]);
    const [namespace, setNamespace] = useState(null);
    const [configFilePath, setConfigFilePath] = useState(null);
    const [loading, setLoading] = useState(true);
    const [websites, setWebsites] = useState([]);

    useEffect(() => {
        window.backend.Client.ListNamespaces().then((r) => {
            setNamespaces(r);
            setNamespace("default");
        });
    }, []);

    useEffect(() => {
        if (namespace !== null) {
            setLoading(true)
            window.backend.Client.GetWebsitesInNamespace(namespace).then(results => {
                setWebsites(JSON.parse(results));
                setLoading(false);
            });
        }
    }, [namespace]);


    return (
        <div id="app" className="App">
            <AppBar id="Controls">
                <Toolbar>
                    <Typography className={classes.title} variant="h6" noWrap>
                        Portfall
                    </Typography>
                    <FormControl className={classes.formControl} color="inherit">
                        {/* todo: multiple namespace selection */}
                        <Autocomplete options={[""].concat(namespaces)} getOptionLabel={option => option || "All"}
                                      getOptionValue={option => option}
                                      style={{width: 250, color: 'white'}}
                                      value={namespace}
                                      onChange={(_, value) => {
                                          setNamespace(value)
                                      }}
                                      renderInput={params => <TextField {...params} style={{color: 'white'}}
                                                                        label="Namespaces"
                                                                        variant="outlined"/>}/>
                    </FormControl>
                </Toolbar>
            </AppBar>
            <div id="Websites" style={{paddingTop: 100, flex: 1, alignItems: 'center', justifyContent: 'center', display: 'flex'}}>
                {loading ? <CircularProgress /> : (
                    <Grid style={{
                        flexGrow: 1,
                        padding: "0 2rem"
                    }}>
                        {websites.length === 0 ? <Alert icon={<MoodBadTwoTone/>} severity="info"><Typography>No websites found to port-forward in this namespace</Typography></Alert>  : null}
                        {websites.map(({localPort, podPort, title, iconRemoteUrl}) => (
                            <Grid item xs={4}>
                                <Card>
                                    <CardHeader avatar={<Avatar src={iconRemoteUrl}></Avatar>} title={title}/>
                                    <CardContent>
                                        <Typography color="textSecondary">pod {podPort} {'<->'} local {localPort}</Typography>
                                    </CardContent>
                                    <CardActions>
                                        <Button size="small" color="primary" onClick={() => window.backend.OpenInBrowser(`http://localhost:${localPort}`)}>
                                            Open
                                        </Button>
                                    </CardActions>
                                </Card>
                            </Grid>
                        ))}

                    </Grid>
                )}
            </div>
        </div>
    );
}

export default App;
