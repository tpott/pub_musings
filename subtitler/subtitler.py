# subtitler.py
# Trevor Pottinger
# Tue Apr 28 19:50:37 PDT 2020

import argparse
import base64
import json
import os
import subprocess
from typing import List
import urllib.parse


def urandom5() -> str:
  """Reads 5 bytes from /dev/urandom and encodes them in lowercase base32"""
  with open('/dev/urandom', 'rb') as f:
    return base64.b32encode(f.read(5)).decode('utf-8').lower()


def youtubeVideoID(url: str) -> str:
  """Given a URL, validate that its for Youtube, and return its ID"""
  obj = urllib.parse.urlparse(url)
  assert obj.scheme == 'https'
  assert obj.netloc == 'www.youtube.com'
  data = urllib.parse.parse_qs(obj.query)
  assert 'v' in data, 'Expected a "v" param, like "?v=SOME_ID"'
  assert len(data['v']) == 1, 'Expected a single "v" value, got %d values' % len(data['v'])
  return data['v'][0]


def mysystem(command: List[str]) -> int:
  if True:
    print(command)
    return 0
  res = subprocess.run(command)
  return res.returncode


def run(url: str, filename: str, lang: str) -> None:
  bucket = 'subtitler1'
  region = 'us-east-2'

  job_id = urandom5()
  video_id = youtubeVideoID(url)
  job_name = 'test-{job_id}-{video_id}'.format(
    job_id=job_id,
    video_id=video_id
  )

  _resp = mysystem(['youtube-dl', url, '--output', 'downloads/{video_id}.%(ext)s'.format(
    video_id=video_id
  )])

  files = [f for f in os.listdir('downloads') if f.startswith(video_id)]
  assert len(files) > 0, 'Expected at least one video file like %s.*' % video_id
  video_file = files[0]

  # supported file types: mp3 | mp4 | wav | flac
  # from https://docs.aws.amazon.com/transcribe/latest/dg/API_TranscriptionJob.html#transcribe-Type-TranscriptionJob-MediaFormat
  _resp = mysystem([
    'ffmpeg',
    '-i',
    'downloads/{video_file}'.format(video_file=video_file),
    'audios/{video_id}.wav'.format(video_id=video_id),
  ])

  # Result should be https://s3.console.aws.amazon.com/s3/buckets/subtitler1/?region=us-east-2
  _resp = mysystem([
    'aws',
    's3',
    'cp',
    'audios/{video_id}.wav'.format(video_id=video_id),
    's3://{bucket}/'.format(bucket=bucket),
  ])

  job_obj = {
    'TranscriptionJobName': job_name,
    'LanguageCode': lang,
    'MediaFormat': 'wav',
    'Media': {
      'MediaFileUri': 'https://{bucket}.s3.{region}.amazonaws.com/{filename}'.format(
        bucket=bucket,
        filename=filename,
        region=region,
      ),
    },
  }
  json_job_str = json.dumps(job_obj)
  with open('job-start-command.json', 'wb') as f:
    f.write(json_job_str.encode('utf-8'))
    f.write(b'\n')

  _resp = mysystem([
    'aws',
    'transcribe',
    'start-transcription-job',
    '--region',
    region,
    '--cli-input-json',
    'file://job-start-command.json',
  ])

  # TODO do while in-progress
  # Note: `--status IN_PROGRESS` is optional
  _resp = mysystem([
    'aws',
    'transcribe',
    'list-transcription-jobs',
    '--region',
    region,
    '--status',
    'IN_PROGRESS',
  ])

  # TODO split into two commands
  _resp = os.system('curl -o output.json "$(aws transcribe get-transcription-job --region {region} --transcription-job-name {job_name} | jq -r .TranscriptionJob.Transcript.TranscriptFileUri)"'.format(job_name=job_name, region=region))

  # jq -Cc '.results.items[]' output.json | head
  # jq -cr '.results.items[] | [.start_time, .end_time, .alternatives[0].content] | @tsv' output.json | head
  # TODO format the SUB file based on the JSON output
  # https://wiki.videolan.org/SubViewer/
  sub_format = """
  [INFORMATION]
  [TITLE] {video_title}
  [END INFORMATION]

  00:00:00.00,00:00:10.00
  some text
  """ # hours:minutes:seconds.centiseconds
  # timestart,timeend
  # TODO ffmpeg -i {filename} -i formatted.srt -c copy -c:s mov_text final_{filename}
  print('ffmpeg -i downloads/{video_file} -i formatted.srt -c copy -c:s mov_text final/{filename}')
  return
  # end def run


# `aws configure list` should show an access key and secret key. Else try
# https://console.aws.amazon.com/iam/home?region=us-east-2#/users/test-user1?section=security_credentials

def main() -> None:
  """
  Dependencies:
  * youtube-dl
  * ffmpeg
  * awscli, version >= 1.18 (probably install via pip and export PATH=$PATH:~/.local/bin)
  * [jq]
  """
  parser = argparse.ArgumentParser('Download and transcribe a video from youtube')
  parser.add_argument('video_url')
  parser.add_argument('temp_video_file')
  parser.add_argument('language')
  args = parser.parse_args()
  language = args.language.lower()
  assert language in ['english', 'hindi']
  lang = 'en-US'
  if language == 'hindi':
    lang = 'hi-IN'
  run(args.video_url, args.temp_video_file, lang)
  return


if __name__ == '__main__':
  main()
