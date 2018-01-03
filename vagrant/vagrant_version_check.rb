# -*- mode: ruby -*-
# vi: set ft=ruby :

MINIMUM_VAGRANT_VERSION='1.7.3'

if Vagrant::VERSION < MINIMUM_VAGRANT_VERSION
  STDERR.puts "netplugin requires Vagrant #{MINIMUM_VAGRANT_VERSION} or greater. You are running #{Vagrant::VERSION}"
  STDERR.puts 'Aborting!'
  exit 1
end
