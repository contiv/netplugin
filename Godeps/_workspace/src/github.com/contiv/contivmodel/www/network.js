// network.js
// Display Network information

var contivModel = require("../client/contivModel")

var NetworkPane = React.createClass({
  	render: function() {
		var self = this

		if (self.props.networks === undefined) {
			return <div> </div>
		}
        return (
            <div style={{margin: '5%',}}>
                <NetworkSummaryView key="NetworkSummary" networks={self.props.networks}/>
            </div>
        );
	}
});

var NetworkSummaryView = React.createClass({
  	render: function() {
		var self = this

		// Walk thru all objects
		var networkListView = self.props.networks.map(function(network){
			return (
				<ModalTrigger modal={<NetworkModalView network={ network }/>}>
					<tr key={ network.key } className="info">
                        <td>{ network.networkName }</td>
                        <td>{ network.encap }</td>
                        <td>{ network.subnet }</td>
                        <td>{ network.gateway }</td>

					</tr>
				</ModalTrigger>
			);
		});

		return (
        <div>
			<Table hover>
				<thead>
					<tr>
                        <th> Network name </th>
                        <th> Encapsulation </th>
                        <th> Subnet </th>
						<th> Gateway </th>
					</tr>
				</thead>
				<tbody>
            		{ networkListView }
				</tbody>
			</Table>
        </div>
    	);
	}
});

var NetworkModalView = React.createClass({
	render() {
		var obj = this.props.network
	    return (
	      <Modal {...this.props} bsStyle='primary' bsSize='large' title='Network' animation={false}>
	        <div className='modal-body' style={ {margin: '5%',} }>
                <Input type='text' label='Tenant Name' ref='tenantName' defaultValue={obj.tenantName} placeholder='Tenant Name' />
                <Input type='text' label='Network name' ref='networkName' defaultValue={obj.networkName} placeholder='Network name' />
				<Input type='text' label='Encapsulation' ref='encap' defaultValue={obj.encap} placeholder='Encapsulation' />
				<Input type='text' label='Subnet' ref='subnet' defaultValue={obj.subnet} placeholder='Subnet' />
                <Input type='text' label='Gateway' ref='defaultGw' defaultValue={obj.gateway} placeholder='Gateway' />
			</div>
	        <div className='modal-footer'>
				<Button onClick={this.props.onRequestHide}>Close</Button>
	        </div>
	      </Modal>
	    );
  	}
});


module.exports = NetworkPane
