// volumes.js
// Display Volumes information

var VolumesPane = React.createClass({
  	render: function() {
		var self = this

		if (self.props.volumes === undefined) {
			return <div> </div>
		}

		// Walk thru all the volumes
		var volListView = self.props.volumes.map(function(vol){
			return (
				<tr key={vol.key} className="info">
					<td>{vol.tenantName}</td>
					<td>{vol.volumeName}</td>
					<td>{vol.poolName}</td>
					<td>{vol.size}</td>
				</tr>
			);
		});

		// Render the pane
		return (
        <div style={{margin: '5%',}}>
			<Table hover>
				<thead>
					<tr>
						<th>Tenant</th>
						<th>Volume</th>
						<th>Pool</th>
						<th>Size</th>
					</tr>
				</thead>
				<tbody>
            		{volListView}
				</tbody>
			</Table>
        </div>
    );
	}
});

module.exports = VolumesPane
