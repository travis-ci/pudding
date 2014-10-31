package workers

import "text/template"

var (
	initScript = template.Must(template.New("init-script").Parse(`#!/bin/bash
set -o errexit

export INSTANCE_ID=$(curl -s 'http://169.254.169.254/latest/meta-data/instance-id')

curl -s -f \
  -d "state=started&instance-id=$INSTANCE_ID&slack-channel={{.SlackChannel}}" \
  -X PATCH \
  "{{.InstanceBuildURL}}?l=cloud-init-$LINENO&m=started"

cd /tmp

export TRAVIS_WORKER_HOST_NAME="worker-linux-docker-${INSTANCE_ID#i-}.{{.Env}}.travis-ci.{{.Site}}"

cat > docker_rsa <<EOF
{{.DockerRSA}}
EOF

cat > travis-worker.yml <<EOF
{{.TravisWorkerYML}}
EOF

cat > papertrail.conf <<EOF
\$DefaultNetstreamDriverCAFile /etc/papertrail.crt
\$DefaultNetstreamDriver gtls
\$ActionSendStreamDriverMode 1
\$ActionSendStreamDriverAuthMode x509/name

*.* @@{{.PapertrailSite}}
EOF

cat > watch-files.conf <<EOF
\$ModLoad imfile
\$InputFileName /etc/sv/travis-worker/log/main/current
\$InputFileTag travis-worker
\$InputFileStateFile state_file_worker_log
\$InputFileFacility local7
\$InputRunFileMonitor
\$InputFilePollInterval 10
EOF

curl -s -f \
  -d "state=started&instance-id=$INSTANCE_ID&slack-channel={{.SlackChannel}}" \
  -X PATCH \
  "{{.InstanceBuildURL}}?l=cloud-init-$LINENO&m=pre-install"

sed -i -e "s/^Hostname.*$/Hostname \"$TRAVIS_WORKER_HOST_NAME\"/" /etc/collectd/collectd.conf
mkdir /home/deploy/.ssh
chown travis:travis /home/deploy/.ssh
chmod 0700 /home/deploy/.ssh
mv docker_rsa /home/deploy/.ssh/docker_rsa
chown travis:travis /home/deploy/.ssh/docker_rsa
chmod 0600 /home/deploy/.ssh/docker_rsa
mv travis-worker.yml /home/deploy/travis-worker/config/worker.yml
chown travis:travis /home/deploy/travis-worker/config/worker.yml
chmod 0600 /home/deploy/travis-worker/config/worker.yml
mv watch-files.conf /etc/rsyslog.d/60-watch-files.conf
mv papertrail.conf /etc/rsyslog.d/65-papertrail.conf
service rsyslog restart

rm -rf /var/lib/cloud/instances/*
curl -s -f \
  -d "state=finished&instance-id=$INSTANCE_ID&slack-channel={{.SlackChannel}}" \
  -X PATCH \
  "{{.InstanceBuildURL}}?l=cloud-init-$LINENO&m=finished"
`))
)

type initScriptContext struct {
	Env              string
	Site             string
	DockerRSA        string
	SlackChannel     string
	PapertrailSite   string
	TravisWorkerYML  string
	InstanceBuildID  string
	InstanceBuildURL string
}
