// groups.js
// Display Endpoint group information

var contivModel = require("../client/contivModel")

var GroupsPane = React.createClass({
  	render: function() {
		var self = this

		if (self.props.endpointGroups === undefined) {
			return <div> </div>
		}

        var EndpointGroupSummaryView = contivModel.EndpointGroupSummaryView
        return (
            <div style={{margin: '5%',}}>
                <EndpointGroupSummaryView key="EndpointGroupSummary" endpointGroups={self.props.endpointGroups}/>
            </div>
        )
	}
});

module.exports = GroupsPane
