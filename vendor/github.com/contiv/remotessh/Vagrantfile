Vagrant.configure(2) do |config|
  config.vm.provider 'virtualbox' do |v|
    v.linked_clone = true if Vagrant::VERSION =~ /^1.8/
  end
  config.vm.box = "contiv/centos72"
  (0..2).each do |x|
    config.vm.define "host#{x}" do |host|
    config.vm.box_version = "0.7.0"
    # use a private key from within the repo for demo environment. This is used for
    # baremetal test
    config.ssh.insert_key = false
    config.ssh.private_key_path = "./testdata/insecure_private_key"
    end
  end
end
