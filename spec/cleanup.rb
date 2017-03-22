#!/usr/bin/env ruby
# encoding: utf-8

require "nokogiri"

FILENAME = "docs/contiv.html"

doc = Nokogiri::HTML(File.read(FILENAME))

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

# overwrite the original HTML file

File.write(FILENAME, doc.to_html)
