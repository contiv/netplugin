// navTab.js
// Navigation tab

// panels
var HomePane = require("./home")
var NetworkPane = require("./network")
var GroupsPane = require("./groups")
var PolicyPane = require("./policy")
var VolumesPane = require("./volumes")

window.globalRefreshDelay = 2000

// Define tabs
var ControlledTabArea = React.createClass({
  getInitialState: function() {
    return {
      key: 1,
    };
  },

  getStateFromServer: function() {
    // Sort function for all contiv objects
    var sortObjFunc = function(first, second) {
      if (first.key > second.key) {
          return 1
      } else if (first.key < second.key) {
          return -1
      }

      return 0
    }

    // Get all endpoints
    $.ajax({
      url: "/endpoints",
      dataType: 'json',
      success: function(data) {

        // Sort the data
        data = data.sort(sortObjFunc);

        this.setState({endpoints: data});

        // Save it in a global variable for debug
        window.globalEndpoints = data
      }.bind(this),
      error: function(xhr, status, err) {
        // console.error("/endpoints", status, err.toString());
        this.setState({endpoints: []});
      }.bind(this)
    });

    // Get all networks
    $.ajax({
      url: "/api/networks/",
      dataType: 'json',
      success: function(data) {

        // Sort the data
        data = data.sort(sortObjFunc);

        this.setState({networks: data});

        // Save it in a global variable for debug
        window.globalNetworks = data
      }.bind(this),
      error: function(xhr, status, err) {
        console.error("/api/networks/", status, err.toString());
      }.bind(this)
    });

    // Get all endpoint groups
    $.ajax({
      url: "/api/endpointGroups/",
      dataType: 'json',
      success: function(data) {

        // Sort the data
        data = data.sort(sortObjFunc);

        this.setState({endpointGroups: data});

        // Save it in a global variable for debug
        window.globalEndpointGroups = data
      }.bind(this),
      error: function(xhr, status, err) {
        console.error("/api/endpointGroups/", status, err.toString());
      }.bind(this)
    });

    // Get all policies
    $.ajax({
      url: "/api/policys/",
      dataType: 'json',
      success: function(data) {

        // Sort the data
        data = data.sort(sortObjFunc);

        this.setState({policies: data});

        // Save it in a global variable for debug
        window.globalPolicies = data
      }.bind(this),
      error: function(xhr, status, err) {
        console.error("/api/policys/", status, err.toString());
      }.bind(this)
    });

    // Get all rules
    $.ajax({
      url: "/api/rules/",
      dataType: 'json',
      success: function(data) {

        // Sort the data
        data = data.sort(sortObjFunc);

        this.setState({rules: data});

        // Save it in a global variable for debug
        window.globalRules = data
      }.bind(this),
      error: function(xhr, status, err) {
        console.error("/api/rules/", status, err.toString());
      }.bind(this)
    });
  },
  componentDidMount: function() {
    this.getStateFromServer();

    // Get state every 2 sec
    setInterval(this.getStateFromServer, window.globalRefreshDelay);
  },
  handleSelect: function(key) {
    this.setState({key: key});
  },

  render: function() {
      var self = this

    return (
      <TabbedArea activeKey={this.state.key} onSelect={this.handleSelect}>
        <TabPane eventKey={1} tab='Home'>
            <HomePane key="home" endpoints={this.state.endpoints} />
        </TabPane>
        <TabPane eventKey={3} tab='Networks'> <h3> Networks </h3>
            <NetworkPane key="networks" networks={this.state.networks} />
        </TabPane>
        <TabPane eventKey={4} tab='Groups'> <h3> Groups </h3>
            <GroupsPane key="groups" endpointGroups={this.state.endpointGroups} />
        </TabPane>
        <TabPane eventKey={5} tab='Policies'> <h3> Policy </h3>
            <PolicyPane key="policy" policies={this.state.policies} />
        </TabPane>
        <TabPane eventKey={6} tab='Volumes'> <h3> Volumes </h3>
            <VolumesPane key="volumes" volumes={this.state.volumes} />
        </TabPane>
      </TabbedArea>
    );
  }
});

module.exports = ControlledTabArea
