pkill -f serverlinux
sleep 2
cp /www/wwwroot/goo/serverlinux.new /www/wwwroot/goo/serverlinux
chmod +x /www/wwwroot/goo/serverlinux
cd /www/wwwroot/goo
nohup ./serverlinux >/dev/null 2>&1 &
sleep 1
pgrep -a serverlinux
