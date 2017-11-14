import React, { Component } from 'react';
import {
    Table,
    TableBody,
    TableFooter,
    TableHeader,
    TableHeaderColumn,
    TableRow,
    TableRowColumn,
} from 'material-ui/Table';
import ActionSubject from 'material-ui/svg-icons/action/subject';
import IconButton from 'material-ui/IconButton';
import FlatButton from 'material-ui/FlatButton';

import Dialog from 'material-ui/Dialog';

import { getPodLogs } from '../services'
let text = "";

class PodList extends Component {

    constructor(props) {
        super(props);
        this.state = {
            isLogModalOpened: false,
            logs: "hola"
        }
    }

    actions = [
        <FlatButton
            label="Close"
            primary={true}
            keyboardFocused={true}
            onClick= {_ =>  this.setState({isLogModalOpened: false})} />,
    ];

    render() {
        let pods = this.props.pods;
        return (
            <div>
                <Table
                    selectable={false}
                    multiSelectable={false}>
                    <TableHeader
                        displaySelectAll={false}
                        adjustForCheckbox={false}>
                        <TableRow>
                            <TableHeaderColumn>Name</TableHeaderColumn>
                            <TableHeaderColumn>Status</TableHeaderColumn>
                            <TableHeaderColumn>Logs</TableHeaderColumn>
                        </TableRow>
                    </TableHeader>
                    <TableBody displayRowCheckbox={false}>
                        {pods.map((p, i) => (
                            <TableRow key={i}>
                                <TableRowColumn style={{ fontWeight: "bold" }}>{p.metadata.name}</TableRowColumn>
                                <TableRowColumn>{p.status.phase}</TableRowColumn>
                                <TableRowColumn>
                                    <IconButton><ActionSubject onClick={_ => this.getLogsForPod(p)} /></IconButton>
                                </TableRowColumn>
                            </TableRow>
                        ))}
                    </TableBody>
                </Table>
                <Dialog
                    title="Logs"
                    actions={this.actions}
                    modal={true}
                    open={this.state.isLogModalOpened}
                    autoScrollBodyContent={true}>
                    <p style={{whiteSpace: "pre-wrap", backgroundColor: "black", color: "white"}}>
                        {this.state.logs}
                    </p>
                </Dialog>
            </div>
        )
    }

    getLogsForPod(pod) {
        getPodLogs(pod.metadata.namespace, pod.metadata.name)
            .then(b => {
                this.setState({logs: b, isLogModalOpened: true});
            })
            .catch(b => console.error)

    }
}


export default PodList