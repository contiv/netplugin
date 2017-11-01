#!groovy
pipeline {
  agent { label 'public' }
  options {
    timeout(time: 60, unit: 'MINUTES')
  }
  stages {
    stage('Tests') {
      parallel {
        stage('Base') {
          agent { label 'public' }
          steps {
            sh '''
              set -euo pipefail
              export GOPATH="$WORKSPACE"
              export GOBIN="$GOPATH/bin"
              export GOSRC="$GOPATH/src"
              export PATH="$PATH:/sbin/:/usr/local/go/bin:$GOBIN:$GOPATH/netplugin/bin"
              make all-CI
            '''
          }
          post {
            always {
              sh '''
                set -euo pipefail
                export GOPATH="$WORKSPACE"
                export GOBIN="$GOPATH/bin"
                export GOSRC="$GOPATH/src"
                export PATH="$PATH:/sbin/:/usr/local/go/bin:$GOBIN:$GOPATH/netplugin/bin"
                ./scripts/jenkins_cleanup.sh
              '''
            }
          }
        }
        stage('L3') {
          agent { label 'public' }
          steps {
            sh '''
              set -euo pipefail
              export GOPATH="$WORKSPACE"
              export GOBIN="$GOPATH/bin"
              export GOSRC="$GOPATH/src"
              export PATH="$PATH:/sbin/:/usr/local/go/bin:$GOBIN:$GOPATH/netplugin/bin"
              make l3-test
            '''
          }
          post {
            always {
              sh '''
                set -euo pipefail
                export GOPATH="$WORKSPACE"
                export GOBIN="$GOPATH/bin"
                export GOSRC="$GOPATH/src"
                export PATH="$PATH:/sbin/:/usr/local/go/bin:$GOBIN:$GOPATH/netplugin/bin"
                CONTIV_NODES=3 CONTIV_L3=1 vagrant destroy -f
              '''
            }
          }
        }
        stage('Kubernetes') {
          agent { label 'public' }
          steps {
            sh '''
              set -euo pipefail
              export GOPATH="$WORKSPACE"
              export GOBIN="$GOPATH/bin"
              export GOSRC="$GOPATH/src"
              export PATH="$PATH:/sbin/:/usr/local/go/bin:$GOBIN:$GOPATH/netplugin/bin"
              make k8s-test
            '''
          }
          post {
            always {
              sh '''
                set -euo pipefail
                export GOPATH="$WORKSPACE"
                export GOBIN="$GOPATH/bin"
                export GOSRC="$GOPATH/src"
                export PATH="$PATH:/sbin/:/usr/local/go/bin:$GOBIN:$GOPATH/netplugin/bin"
                make k8s-destroy
              '''
            }
          }
        }
      }
    }
  }
}
