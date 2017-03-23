#!/usr/bin/env ruby
# encoding: utf-8

require "nokogiri"

FILENAME = "docs/contiv.html"

doc = Nokogiri::HTML(File.read(FILENAME))

# ----- IMPORT AUTH_PROXY DATA ----------------------------------------------------------------------

auth_doc = Nokogiri::HTML(File.read("docs/auth_proxy.html"))

# extract the main panel and sidebar link nodes
auth_panel = auth_doc.css(".panel-default").first
sidebar_link = auth_doc.css("#sidebar ul li").first

# add auth_proxy api panel under the header
doc.at(".page-header").after(auth_panel)

# add auth_proxy link to the top of the sidebar
doc.at("#sidebar ul").prepend_child(sidebar_link)

# ----- TIDY OUTPUT FILE ----------------------------------------------------------------------------

# add trailing slashes to all paths
node_groups = [
  doc.css(".panel-title a").select { |n| n.text.start_with?("/") },
  doc.css(".modal-title"),
]

node_groups.flatten.each do |node|
  node.children.last.content = node.children.last.text + "/"
end

# insert additional <head> tag requirements for the contiv header
doc.at("head") << Nokogiri::HTML::DocumentFragment.parse(File.read("docs/head.html"))

# insert the contiv header html into the top of the <body>
doc.at("body").prepend_child(Nokogiri::HTML::DocumentFragment.parse(File.read("docs/body.html")))

# ----- OUTPUT --------------------------------------------------------------------------------------

# overwrite the original HTML file
File.write(FILENAME, doc.to_html)
