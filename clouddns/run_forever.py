# run_forever.py

import json
import subprocess
import sys
import time

SLEEP = 5 # seconds

def main() -> None:
    sleep = SLEEP
    if len(sys.argv) == 2:
        sleep = int(sys.argv[1])
    print(f'will be sleeping every {sleep} seconds')

    while True:
        print(f'checking at {time.time()}')

        try:
            res = subprocess.run(['curl', 'https://cloudflare.com/cdn-cgi/trace'], capture_output=True, text=True, check=True)
            # this is better than `grep ip=`
            current_ip = [ line.lstrip('ip=') for line in res.stdout.split('\n') if line.startswith('ip=') ][0]
        except:
            print('failed to curl cloudflare trace')
            time.sleep(sleep)
            continue

        domain = open('domain.txt').read().strip()
        res = subprocess.run(['dig', '+short', domain, 'a'], capture_output=True, text=True, check=True)
        resolved_ip = res.stdout.strip() # strip trailing '\n'

        if current_ip == resolved_ip:
            print(f'same ip: {current_ip}')
            time.sleep(sleep)
            continue

        api_key = open('api_key.txt').read().strip()
        zone_id = open('zone_id.txt').read().strip()
        res = subprocess.run([
            'curl',
            '--request', 'GET',
            '--url', f'https://api.cloudflare.com/client/v4/zones/{zone_id}/dns_records?name={domain}',
            '--header', 'Content-Type: application/json',
            '--header', f'Authorization: Bearer {api_key}',
        ], capture_output=True, text=True, check=True)
        try:
            res_obj = json.loads(res.stdout)
            record_id = [ record['id'] for record in res_obj['result'] if record['name'] == domain ][0]
        except Exception as e:
            print('failed to parse cloudflare dns_records response')
            print(e)
            print(res.stdout)
            time.sleep(sleep)
            continue

        data = json.dumps({
            'content': current_ip,
            'name': domain,
            'proxied': False,
            'type': 'A',
            'comment': 'Domain verification record',
            # 'tags': ['owner:dns-team'],
            'ttl': 60,
        })
        api_key = open('api_key.txt').read().strip()
        zone_id = open('zone_id.txt').read().strip()
        res = subprocess.run([
            'curl',
            '--request', 'PUT',
            '--url', f'https://api.cloudflare.com/client/v4/zones/{zone_id}/dns_records/{record_id}',
            '--header', 'Content-Type: application/json',
            '--header', f'Authorization: Bearer {api_key}',
            '--data', data,
        ], capture_output=True, text=True, check=True)
        print(res.stdout)

        print('sleeping a little extra because of the above write')
        time.sleep(60 + sleep)


if __name__ == '__main__':
    main()
