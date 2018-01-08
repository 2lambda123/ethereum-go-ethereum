// @flow

// Copyright 2017 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

import React, {Component} from 'react';

import withStyles from 'material-ui/styles/withStyles';

import Header from './Header';
import Body from './Body';
import Footer from './Footer';
import {MENU} from './Common';
import type {Content} from '../types/content';

// deepCopy retrieves a copy of the given object, copying recursively in all depth.
// It is used at the state update, because React doesn't handle the object nesting well.
// References are prohibited, since the state needs to be immutable.
// NOTE: maybe there is better solution.
const deepCopy = (prev: mixed) => {
	const copied = Array.isArray(prev) ? [] : {};
	Object.keys(prev).forEach((key) => {
		const c: mixed = prev[key]
		copied[key] = typeof c === 'object' ? deepCopy(c) : c;
	});

	return copied;
};

// deepUpdate updates an object corresponding to the given update data, which has
// the shape of the same structure as the original object. handler also has the same
// structure, except that it contains functions where the original data needs to be
// updated. These functions are used to handle the update.
//
// Since the messages have the same shape as the state content, this approach allows
// the generalization of the message handling. The only necessary thing is to set a
// handler function for every path of the state in order to maximalize the flexibility
// of the update.
const deepUpdate = (prev: mixed, update: mixed, handler: mixed) => {
	if (typeof update === 'undefined') {
		return deepCopy(prev);
	}
	if (typeof handler === 'function') {
		return handler(prev, update);
	}
	const updated = Array.isArray(prev) ? [] : {};
	Object.keys(prev).forEach((key) => {
		if (typeof update[key] === 'undefined') {
			updated[key] = deepCopy(prev[key]);
		} else {
			updated[key] = deepUpdate(prev[key], update[key], handler[key]);
		}
	});

	return updated;
};

// shouldUpdate retrieves the structure of a message. It is used to prevent unnecessary render
// method triggerings. In the affected component's shouldComponentUpdate method it can be checked
// whether the involved data was changed or not by checking the message structure.
const shouldUpdate = (msg: mixed, handler: mixed) => {
	const su = {};
	Object.keys(msg).forEach((key) => {
		su[key] = typeof msg[key] === 'object' && typeof handler[key] !== 'function' ? shouldUpdate(msg[key], handler[key]) : true;
	});
	return su;
};

// appender is a state update generalization function, which appends the update data
// to the existing data. limit defines the maximum allowed size of the created array.
const appender = <T>(limit: number) => (prev: Array<T>, update: Array<T>) => [...prev, ...update].slice(-limit);
// appender200 is an appender function with limit 200.
const appender200 = appender(200);
// replacer is a state update generalization function, which replaces the original data.
const replacer = <T>(prev: T, update: T) => update;

// defaultContent is the initial value of the state content.
const defaultContent: Content = {
	general: {
		version: '-',
	},
	home: {
		memory:  [],
		traffic: [],
	},
	chain:   {},
	txpool:  {},
	network: {},
	system:  {},
	logs:    {
		log: [],
	},
};
// handlers contains the state update generalization functions for each path of the state.
// TODO (kurkomisi): Define a tricky type which embraces the content and the handlers.
const handlers = {
	general: {
		version: replacer,
	},
	home: {
		memory:  appender200,
		traffic: appender200,
	},
	chain:   null,
	txpool:  null,
	network: null,
	system:  null,
	logs:    {
		log: appender200,
	},
};
// styles retrieves the styles for the Dashboard component.
const styles = theme => ({
	dashboard: {
		display:    'flex',
		flexFlow:   'column',
		width:      '100%',
		height:     '100%',
		background: theme.palette.background.default,
		zIndex:     1,
		overflow:   'hidden',
	},
});
export type Props = {
	classes: Object,
};
type State = {
	active: string, // active menu
	sideBar: boolean, // true if the sidebar is opened
	content: Content, // the visualized data
	shouldUpdate: Object // labels for the components, which need to rerender based on the incoming message
};
// Dashboard is the main component, which renders the whole page, makes connection with the server and
// listens for messages. When there is an incoming message, updates the page's content correspondingly.
class Dashboard extends Component<Props, State> {
	constructor(props: Props) {
		super(props);
		this.state = {
			active:       MENU.get('home').id,
			sideBar:      true,
			content:      defaultContent,
			shouldUpdate: {},
		};
	}

	// componentDidMount initiates the establishment of the first websocket connection after the component is rendered.
	componentDidMount() {
		this.connect();
	}

	// connect establishes a websocket connection with the server, listens for incoming messages
	// and tries to reconnect on connection loss.
	connect = () => {
		const server = new WebSocket(`${((window.location.protocol === 'https:') ? 'wss://' : 'ws://') + window.location.host}/api`);
		server.onmessage = (event) => {
			const msg: Content = JSON.parse(event.data);
			if (!msg) {
				return;
			}
			this.update(msg);
		};
		server.onclose = () => {
			setTimeout(this.reconnect, 3000);
		};
	};

	// reconnect clears the state before the connection establishment.
	reconnect = () => {
		this.setState({content: defaultContent, shouldUpdate: {}});
		this.connect();
	};

	// update updates the content corresponding to the incoming message.
	update = (msg: $Shape<Content>) => {
		this.setState(prevState => ({
			content:      deepUpdate(prevState.content, msg, handlers),
			shouldUpdate: shouldUpdate(msg, handlers),
		}));
	};

	// changeContent sets the active label, which is used at the content rendering.
	changeContent = (newActive: string) => {
		this.setState(prevState => (prevState.active !== newActive ? {active: newActive} : {}));
	};

	// openSideBar opens the sidebar.
	openSideBar = () => {
		this.setState({sideBar: true});
	};

	// closeSideBar closes the sidebar.
	closeSideBar = () => {
		this.setState({sideBar: false});
	};

	render() {
		const {classes} = this.props; // The classes property is injected by withStyles().

		return (
			<div className={classes.dashboard}>
				<Header
					opened={this.state.sideBar}
					openSideBar={this.openSideBar}
					closeSideBar={this.closeSideBar}
				/>
				<Body
					opened={this.state.sideBar}
					changeContent={this.changeContent}
					active={this.state.active}
					content={this.state.content}
					shouldUpdate={this.state.shouldUpdate}
				/>
				<Footer
					opened={this.state.sideBar}
					openSideBar={this.openSideBar}
					closeSideBar={this.closeSideBar}
					general={this.state.content.general}
					shouldUpdate={this.state.shouldUpdate}
				/>
			</div>
		);
	}
}

export default withStyles(styles)(Dashboard);
