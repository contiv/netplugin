#!/usr/bin/env ruby
# encoding: utf-8

require "nokogiri"

FILENAME = "docs/contiv.html"

doc = Nokogiri::HTML(File.read(FILENAME))

node_groups = [
  doc.css(".panel-title a").select { |n| n.text.start_with?("/") },
  doc.css(".modal-title"),
]

node_groups.flatten.each do |node|
  node.children.last.content = node.children.last.text + "/"
end

File.write(FILENAME, doc.to_html)
