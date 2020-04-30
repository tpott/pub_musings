# subtitler.py
# Trevor Pottinger
# Tue Apr 28 19:50:37 PDT 2020

import argparse
import base64
import json
import math
import os
import subprocess
from typing import (Any, Dict, List, Tuple)
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


def mysystem_wrapper(dry_run: bool, command: List[str]) -> int:
  if dry_run:
    print(" ".join(command))
    return 0
  res = subprocess.run(command)
  return res.returncode


# https://wiki.videolan.org/SubViewer/
def timeize(seconds: float) -> str:
  hours = int(math.floor(seconds / 3600))
  seconds -= hours * 3600
  minutes = int(math.floor(seconds / 60))
  seconds -= minutes * 60
  return "{hours:02d}:{minutes:02d}:{seconds:02.03f}".format(
    hours=hours,
    minutes=minutes,
    seconds=seconds
  )


def textize(index_result_tuple: Tuple[int, Dict[str, Any]]) -> str:
  index, result = index_result_tuple
  end = timeize(float(result['end_time']))
  start = timeize(float(result['start_time']))
  text = result['alternatives'][0]['content']
  return """{index}
{start} --> {end}
{text}""".format(index=index + 1, start=start, end=end, text=text)


def gen_subtitles(url: str, filename: str, lang: str, dry_run: bool) -> None:
  bucket = 'subtitler1'
  region = 'us-east-2'

  mysystem = lambda command: mysystem_wrapper(dry_run, command)

  job_id = urandom5()
  video_id = youtubeVideoID(url)
  job_name = 'test-{job_id}-{video_id}'.format(
    job_id=job_id,
    video_id=video_id
  )
  print('Running job_name: {job_name}'.format(job_name=job_name))

  _resp = mysystem(['youtube-dl', url, '--output', 'downloads/{video_id}.%(ext)s'.format(
    video_id=video_id
  )])

  files = [f for f in os.listdir('downloads') if f.startswith(video_id)]
  if dry_run and len(files) == 0:
    files = ['{video_id}.dummy.mkv'.format(video_id=video_id)]
  assert len(files) > 0, 'Expected at least one video file like %s.*' % video_id
  video_file = files[0]
  file_parts = video_file.split('.')
  assert len(file_parts) >= 2, 'Expected at least two parts in the video file name'
  file_ext = file_parts[-1]

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
      'MediaFileUri': 'https://{bucket}.s3.{region}.amazonaws.com/{video_id}.wav'.format(
        bucket=bucket,
        video_id=video_id,
        region=region,
      ),
    },
  }
  json_job_str = json.dumps(job_obj)
  if not dry_run:
    with open('job-start-command.json', 'wb') as f:
      f.write(json_job_str.encode('utf-8'))
      f.write(b'\n')
  else:
    print('echo \'{json_job_str}\' > job-start-command.json'.format(json_job_str=json_job_str))

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
  command = 'curl -o outputs/{video_id}.json "$(aws transcribe get-transcription-job --region {region} --transcription-job-name {job_name} | jq -r .TranscriptionJob.Transcript.TranscriptFileUri)"'.format(
    job_name=job_name,
    region=region,
    video_id=video_id
  )
  if dry_run:
    print(command)
  else:
    _resp = os.system(command)

  # jq -Cc '.results.items[]' output.json | head
  # jq -cr '.results.items[] | select(.start_time != null) | [.start_time, .end_time, .alternatives[0].content] | @tsv' output.json | head

  result_obj = {}
  if os.path.exists('outputs/{video_id}.json'.format(video_id=video_id)):
    with open('outputs/{video_id}.json'.format(video_id=video_id), 'rb') as f:
      result_obj = json.loads(f.read().decode('utf-8'))

  try:
    results = [item for item in result_obj['results']['items']
               if 'start_time' in item]
  except KeyError as e:
    print('%s: %s' % (type(e).__name__, str(e)))
    return

  subtitle_file = 'subtitles/{video_id}.srt'.format(video_id=video_id)
  text = "\n\n".join(list(map(textize, enumerate(results))))
  with open(subtitle_file, 'wb') as f:
    f.write(text.encode('utf-8'))

  _resp = mysystem([
    'ffmpeg',
    '-i',
    'downloads/{video_file}'.format(video_file=video_file),
    '-i',
    subtitle_file,
    '-s',
    '720x480',
    # '-c',
    # 'copy',
    '-c:s',
    'mov_text',
    'final/{filename}.mp4'.format(filename=filename),
  ])
  return
  # end def gen_subtitles


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
  parser.add_argument('--dry-run', action='store_true', help='Only print commands ' +
                      'that would have run')
  args = parser.parse_args()
  language = args.language.lower()
  assert language in ['english', 'hindi']
  lang = 'en-US'
  if language == 'hindi':
    lang = 'hi-IN'
  gen_subtitles(args.video_url, args.temp_video_file, lang, args.dry_run)
  return
  # end def main


if __name__ == '__main__':
  main()
