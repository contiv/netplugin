// home.js
// Display Endpoint information

var HomePane = React.createClass({
  	render: function() {
		var self = this

		if (self.props.endpoints === undefined) {
            return (
            <div style={{margin: '5%',}}>
    			<Table hover>
    				<thead>
    					<tr>
    						<th>Host</th>
                            <th>Service</th>
    						<th>Network</th>
    						<th>IP address</th>
                            <th> Link </th>
    					</tr>
    				</thead>
    				<tbody>
    				</tbody>
    			</Table>
            </div>
            );
		}

		// Walk thru all the endpoints
		var epListView = self.props.endpoints.map(function(ep){
            var homeUrl = "/proxy/" + ep.ipAddress
			return (
				<tr key={ep.id} className="info">
					<td>{ep.homingHost}</td>
                    <td>{ep.serviceName}</td>
                    <td>{ep.netID}</td>
					<td>{ep.ipAddress}</td>
                    <td> <a href={homeUrl}>{ep.ipAddress}</a></td>
				</tr>
			);
		});

		// Render the pane
		return (
        <div style={{margin: '5%',}}>
			<Table hover>
				<thead>
					<tr>
						<th>Host</th>
                        <th>Service</th>
						<th>Network</th>
						<th>IP address</th>
                        <th> Link </th>
					</tr>
				</thead>
				<tbody>
            		{epListView}
				</tbody>
			</Table>
        </div>
        );
	}
});

module.exports = HomePane
