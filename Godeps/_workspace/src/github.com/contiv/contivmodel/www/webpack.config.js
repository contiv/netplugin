module.exports = {
    entry: "./main.js",
    output: {
        path: __dirname + "/dist",
        filename: "bundle.js"
    },
    module: {
        loaders: [
            { test: /\.css$/, loader: "style!css" },
            {
                //tell webpack to use jsx-loader for all *.jsx files
                test: /\.js$/,
                loader: 'jsx-loader?insertPragma=React.DOM&harmony'
            }
        ]
    }
};
