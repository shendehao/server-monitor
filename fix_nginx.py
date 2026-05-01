import re

conf_path = '/www/server/panel/vhost/nginx/go_serverlinux.conf'
with open(conf_path, 'r') as f:
    content = f.read()

# Find and replace the HTTP_TO_HTTPS block
pattern = r'(    #HTTP_TO_HTTPS_START\n)(.*?)(    #HTTP_TO_HTTPS_END)'
replacement = r'''    #HTTP_TO_HTTPS_START
    set $redirect_to_https 0;
    if ($server_port !~ 443){
        set $redirect_to_https 1;
    }
    if ($uri ~* ^/(api|ws)/){
        set $redirect_to_https 0;
    }
    if ($redirect_to_https = 1){
        rewrite ^(/.*)$ https://$host$1 permanent;
    }
    #HTTP_TO_HTTPS_END'''

new_content, count = re.subn(pattern, replacement, content, flags=re.DOTALL)
if count > 0:
    with open(conf_path, 'w') as f:
        f.write(new_content)
    print(f'OK: replaced {count} block(s)')
else:
    print('ERROR: HTTP_TO_HTTPS block not found')
    # Show what we have
    for line in content.split('\n'):
        if 'HTTP_TO_HTTPS' in line or 'server_port' in line:
            print(f'  > {line}')
