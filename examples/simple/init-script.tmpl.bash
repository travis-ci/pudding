#!/bin/bash

set -o errexit

export INSTANCE_ID="$(curl -s 'http://169.254.169.254/latest/meta-data/instance-id')"

mkdir -p /app
cd /app

cat > id_rsa <<EOF
{{ env_for `RSA_KEY` `site` `env` | uncompress }}
EOF

cat > start-hook <<EOF
#!/bin/bash
exec curl \\
  -s \\
  -X POST \\
  -d '{"instance_id":"$INSTANCE_ID"}' \\
  "{{ .InstanceLaunchURL }}?l=cloud-init-$LINENO&slack-channel={{ .SlackChannel }}"
EOF

chown app:app start-hook
chmod +x start-hook

cat > stop-hook <<EOF
#!/bin/bash
exec curl \\
  -s \\
  -X POST \\
  -d '{"instance_id":"$INSTANCE_ID"}' \\
  "{{ .InstanceTerminateURL }}?l=cloud-init-$LINENO&slack-channel={{ .SlackChannel }}"
EOF

chown app:app stop-hook
chmod +x stop-hook
