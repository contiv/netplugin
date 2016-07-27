Vagrant.configure(2) do |config|
  config.vm.box = "contiv/centos71-netplugin"
  (0..2).each do |x|
    config.vm.define "host#{x}" do |host|
    config.vm.box_version = "0.5.1"
    # use a private key from within the repo for demo environment. This is used for
    # baremetal test
    config.ssh.insert_key = false
    config.ssh.private_key_path = "./testdata/insecure_private_key"
    end
  end
end
