// policy.js
// Display Policy information

var contivModel = require("../client/contivModel")

var PolicySummaryView = React.createClass({
  	render: function() {
		var self = this

		// Walk thru all objects
		var policyListView = self.props.policys.map(function(policy){
			return (
				<ModalTrigger modal={<PolicyModalView policy={ policy }/>}>
					<tr key={ policy.key } className="info">
						<td>{ policy.tenantName }</td>
                        <td>{ policy.policyName }</td>
					</tr>
				</ModalTrigger>
			);
		});

		return (
        <div>
			<Table hover>
				<thead>
					<tr>
						<th> Tenant Name </th>
                        <th> Policy Name </th>
					</tr>
				</thead>
				<tbody>
            		{ policyListView }
				</tbody>
			</Table>
        </div>
    	);
	}
});

var PolicyModalView = React.createClass({
	render() {
		var obj = this.props.policy

        // Create incoming rule list
        var inRules = window.globalRules.filter(function(rule){
            if ((rule.tenantName == obj.tenantName) && (rule.policyName == obj.policyName) &&
                rule.direction == "in") {
                return true
            }

            return false
        });

        // create outgoing rule List
        var outRules = window.globalRules.filter(function(rule){
            if ((rule.tenantName == obj.tenantName) && (rule.policyName == obj.policyName) &&
                rule.direction == "out") {
                return true
            }

            return false
        })

	    return (
	      <Modal {...this.props} bsStyle='primary' bsSize='large' title={obj.policyName} animation={false}>
	        <div className='modal-body' style={ {margin: '5%',} }>
				<Input type='text' label='Tenant Name' ref='tenantName' defaultValue={obj.tenantName} placeholder='Tenant Name' />
                <Input type='text' label='Policy Name' ref='policyName' defaultValue={obj.policyName} placeholder='Policy Name' />
			</div>
            <div style={ {margin: '5%',} }>
                <h3> Incoming Rules </h3>
                <RuleSummaryView key="ruleSummary" rules={inRules} direction="in" />
            </div>
            <div style={ {margin: '5%',} }>
                <h3> Outgoing Rules </h3>
                <RuleSummaryView key="ruleSummary" rules={outRules} direction="out" />
            </div>
	        <div className='modal-footer'>
				<Button onClick={this.props.onRequestHide}>Close</Button>
	        </div>
	      </Modal>
	    );
  	}
});

var RuleModalView = contivModel.RuleModalView
var RuleSummaryView = React.createClass({
  	render: function() {
		var self = this

		// Walk thru all objects
		var ruleListView = self.props.rules.map(function(rule){
            var action = "allow"
            if (rule.action == "deny") {
                action = "deny"
            }
            if (self.props.direction == "out") {
                return (
    				<ModalTrigger modal={<RuleModalView rule={ rule }/>}>
    					<tr key={ rule.key } className="info">
                            <td>{ rule.ruleId }</td>
                            <td>{ rule.priority }</td>
    						<td>{ action }</td>
    						<td>{ rule.toEndpointGroup }</td>
                            <td>{ rule.toIpAddress }</td>
                            <td>{ rule.protocol }</td>
    						<td>{ rule.port }</td>
    					</tr>
    				</ModalTrigger>
    			);
            } else {
                return (
    				<ModalTrigger modal={<RuleModalView rule={ rule }/>}>
    					<tr key={ rule.key } className="info">
                            <td>{ rule.ruleId }</td>
                            <td>{ rule.priority }</td>
    						<td>{ action }</td>
    						<td>{ rule.fromEndpointGroup }</td>
                            <td>{ rule.fromIpAddress }</td>
                            <td>{ rule.protocol }</td>
    						<td>{ rule.port }</td>
    					</tr>
    				</ModalTrigger>
    			);
            }

		});

        // Set appropriate heading based on direction
        var groupHdr = "From Group";
        var ipHdr = "From IP Address"
        if (self.props.direction == "out") {
            groupHdr = "To Group"
            ipHdr = "To IP Address"
        }

		return (
        <div>
			<Table hover>
				<thead>
					<tr>
                        <th> Rule Id </th>
                        <th> Priority </th>
						<th> Action </th>
						<th> { groupHdr } </th>
                        <th> { ipHdr } </th>
                        <th> Protocol </th>
						<th> To Port </th>
					</tr>
				</thead>
				<tbody>
            		{ ruleListView }
				</tbody>
			</Table>
        </div>
    	);
	}
});
var PolicyPane = React.createClass({
  	render: function() {
		var self = this

		if (self.props.policies === undefined) {
			return <div> </div>
		}

        // var PolicySummaryView = contivModel.PolicySummaryView
        return (
            <div style={{margin: '5%',}}>
                <PolicySummaryView key="policySummary" policys={self.props.policies}/>
            </div>
        );
	}
});

module.exports = PolicyPane
