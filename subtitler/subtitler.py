# subtitler.py
# Trevor Pottinger
# Tue Apr 28 19:50:37 PDT 2020

import argparse
import base64
import json
import math
import os
import subprocess
import sys
import time
from typing import (Any, Dict, List, Optional, Tuple)
import urllib.parse

try:
  import spleeter
except ImportError:
  spleeter = None

from align_lyrics import align, alignLyrics, normalizeTextContent
from tsv2srt import tsv2srt
from misc import urandom5


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


def mysystem2(dry_run: bool, command: List[str]) -> Optional[str]:
  if dry_run:
    print(" ".join(command))
    return None
  res = subprocess.run(command, stdout=subprocess.PIPE)
  stdout = res.stdout.decode('utf-8').replace('\n', '')
  # Print this to be consistent with mysystem_wrapper...
  # TODO strip all the extra whitespace from aws list-transcription-jobs
  print(stdout)
  return stdout


def downloadVideo(url: str, video_id: str, dry_run: bool) -> str:
  mysystem = lambda command: mysystem_wrapper(dry_run, command)
  # --no-cache-dir is to avoid some 403 errors
  #   see https://github.com/ytdl-org/youtube-dl/issues/6451
  # --no-playlist is in case someone tries passing in a playlist URL
  # --merge-output-format mkv is because sometimes ffmpeg can't merge
  #   the data correctly, https://github.com/ytdl-org/youtube-dl/issues/20515
  _resp = mysystem([
    'youtube-dl',
    '--no-cache-dir',
    '--no-playlist',
    url,
    # '--merge-output-format',
    # 'mkv',
    '--output',
    'downloads/{video_id}.%(ext)s'.format(
      video_id=video_id
    ),
  ])
  files = [f for f in os.listdir('downloads') if f.startswith(video_id)]
  if dry_run and len(files) == 0:
    files = ['{video_id}.dummy.mkv'.format(video_id=video_id)]
  assert len(files) > 0, 'Expected at least one video file like %s.*' % video_id
  return files[0]


def extractAudio(video_file: str, video_id: str, dry_run: bool) -> None:
  mysystem = lambda command: mysystem_wrapper(dry_run, command)
  # supported file types: mp3 | mp4 | wav | flac
  # from https://docs.aws.amazon.com/transcribe/latest/dg/API_TranscriptionJob.html#transcribe-Type-TranscriptionJob-MediaFormat
  _resp = mysystem([
    'ffmpeg',
    '-i',
    'downloads/{video_file}'.format(video_file=video_file),
    'audios/{video_id}.wav'.format(video_id=video_id),
  ])
  return


def maybeSpleeter(video_id: str, dry_run: bool) -> None:
  if spleeter is None:
    print('spleeter is not installed', file=sys.stderr)
  mysystem = lambda command: mysystem_wrapper(dry_run, command)
  _resp = mysystem([
    'python',
    '-m',
    'spleeter',
    'separate',
    '-i',
    'audios/{video_id}.wav'.format(video_id=video_id),
    '-p',
    'spleeter:2stems',
    '-o',
    'audios/',
  ])
  _resp = mysystem([
    'ffmpeg',
    '-i',
    'audios/{video_id}/vocals.wav'.format(video_id=video_id),
    '-map_channel',
    '0.0.0',
    'audios/{video_id}/vocals_left.wav'.format(video_id=video_id),
    '-map_channel',
    '0.0.1',
    'audios/{video_id}/vocals_right.wav'.format(video_id=video_id),
  ])
  return


def uploadAudioToAws(bucket: str, video_id: str, dry_run: bool) -> None:
  mysystem = lambda command: mysystem_wrapper(dry_run, command)
  # Result should be https://s3.console.aws.amazon.com/s3/buckets/subtitler1/?region=us-east-2
  audio_format = 'audios/{video_id}.wav'
  if spleeter is not None:
    audio_format = 'audios/{video_id}/vocals.wav'
  audio_file = audio_format.format(video_id=video_id)
  _resp = mysystem([
    'aws',
    's3',
    'cp',
    audio_file,
    's3://{bucket}/{video_id}.wav'.format(bucket=bucket, video_id=video_id),
  ])
  return


def startAwsTranscriptJob(
  job_id: str,
  lang: str,
  bucket: str,
  video_id: str,
  region: str,
  dry_run: bool,
) -> None:
  mysystem = lambda command: mysystem_wrapper(dry_run, command)
  job_obj = {
    'TranscriptionJobName': job_id,
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
  json_job_str = json.dumps(job_obj, sort_keys=True)
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
  return
  # end def startAwsTranscriptJob


def waitForAwsTranscriptions(region: str, dry_run: bool) -> None:
  for _ in range(200):
    # Note: `--status IN_PROGRESS` is optional
    res = mysystem2(dry_run, [
      'aws',
      'transcribe',
      'list-transcription-jobs',
      '--region',
      region,
      '--status',
      'IN_PROGRESS',
    ])
    try:
      obj = json.loads(res if res is not None else '')
    except Exception as e:
      print('aws list-transcription-jobs failed: %s: %s' % (type(e).__name__, str(e)))
      break
    try:
      if len(obj['TranscriptionJobSummaries']) == 0:
        break
    except Exception as e:
      print('aws list response was weird: %s: %s' % (type(e).__name__, str(e)))
      break
    time.sleep(3)
  return


def downloadAwsTranscriptions(
  job_id: str,
  region: str,
  video_id: str,
  dry_run: bool,
) -> None:
  # TODO split into two commands
  command = 'curl -o outputs/{video_id}.json "$(aws transcribe get-transcription-job --region {region} --transcription-job-name {job_id} | jq -r .TranscriptionJob.Transcript.TranscriptFileUri)"'.format(
    job_id=job_id,
    region=region,
    video_id=video_id
  )
  if dry_run:
    print(command)
  else:
    _resp = os.system(command)
  return


def uploadAudioToGcp(bucket: str, video_id: str, dry_run: bool) -> None:
  mysystem = lambda command: mysystem_wrapper(dry_run, command)
  # Result should be https://console.cloud.google.com/storage/browser/subtitler2?forceOnBucketsSortingFiltering=false&authuser=1&project=emerald-cacao-282303
  audio_format = 'audios/{video_id}.wav'
  if spleeter is not None:
    # TODO figure out how to pass multi channel to GCP
    audio_format = 'audios/{video_id}/vocals_left.wav'
  audio_file = audio_format.format(video_id=video_id)
  _resp = mysystem([
    'gsutil',
    'cp',
    audio_file,
    'gs://{bucket}/{video_id}.wav'.format(bucket=bucket, video_id=video_id),
  ])
  return


def startGcpTranscriptJob(
  lang: str,
  bucket: str,
  video_id: str,
  dry_run: bool,
) -> None:
  mysystem = lambda command: mysystem_wrapper(dry_run, command)
  _resp = mysystem([
    'gcloud',
    'ml',
    'speech'
    'recognize-long-running',
    'gs://{bucket}/{video_id}.wav'.format(video_id=video_id),
    '--language-code=\'{lang}\''.format(lang=lang),
    '--async',
    '--include-word-time-offsets',
  ])
  print('gcloud response!')
  print(_resp)
  return
  # end def startGcpTranscriptJob


def output2tsv(video_id: str, dry_run: bool) -> None:
  # jq -Cc '.results.items[]' output.json | head
  # jq -cr '.results.items[] | select(.start_time != null) | [.start_time, .end_time, .alternatives[0].content] | @tsv' output.json | head
  # jq -cr '.results.items[] | select(.start_time != null) | [.start_time, .end_time, ((.end_time | tonumber) - (.start_time | tonumber)), (.alternatives[0].content | ascii_downcase)] | @tsv' output.json
  # then pipe to `mlr --itsvlite --otsv label start,end,duration,content then stats1 -a count,sum,mean,min,p25,p50,p75,max -f duration | transpose`
  # where transpose is an alias for:
  # `python3 -c 'from __future__ import (division, print_function); import sys; rows = [line.rstrip("\n").split("\t") for line in sys.stdin]; nrows = len(rows); assert nrows > 0, "Expected at least one row in input"; ncols = len(rows[0]); cols = [[rows[i][j] if j < len(rows[i]) else "" for i in range(nrows)] for j in range(ncols)]; print("\n".join(["\t".join(col) for col in cols]));'`
  # or pipe to `mlr --itsvlite --otsv label start,end,duration,content then filter '$duration > 0.3'`
  #
  # -c is for compact output
  # -r is for "raw" output
  command = 'jq -cr \'.results.items[] | select(.start_time != null) | [.start_time, .end_time, ((.end_time | tonumber) - (.start_time | tonumber)), (.alternatives[0].content | ascii_downcase)] | @tsv\' outputs/{video_id}.json > tsvs/{video_id}.tsv'.format(
    video_id=video_id
  )
  if not dry_run:
    results = []
    with open('outputs/{video_id}.json'.format(video_id=video_id), 'rb') as in_f:
      j_obj = json.loads(in_f.read().decode('utf-8'))
      for item in j_obj['results']['items']:
        if 'start_time' not in item:
          continue
        results.append((
          item['start_time'],
          item['end_time'],
          float(item['end_time']) - float(item['start_time']),
          item['alternatives'][0]['content'].lower()
        ))
    with open('tsvs/{video_id}.tsv'.format(video_id=video_id), 'wb') as out_f:
      for item in results:
        out_f.write('{start}\t{end}\t{duration:0.3f}\t{content}\n'.format(
          start=item[0],
          end=item[1],
          duration=item[2],
          content=normalizeTextContent(item[3])
        ).encode('utf-8'))
  else:
    print(command)
    _resp = os.system(command)
  return
  # end def output2tsv


def alignLyricFile(video_id: str, lyric_file: str, dry_run: bool) -> None:
  if dry_run:
    print('python3 align_lyrics.py tsvs/{video_id}.tsv {lyric_file}'.format(
      lyric_file=lyric_file,
      video_id=video_id
    ))
    return
  if not os.path.isfile(lyric_file):
    print('Lyrics for {lyric_file} don\'t exist'.format(lyric_file=lyric_file), file=sys.stderr)
    return
  # TODO call alignLyrics('tsvs/{video_id}.tsv'.format(video_id=video_id), lyric_file)
  transcribed = []
  with open('tsvs/{video_id}.tsv'.format(video_id=video_id), 'rb') as in_f:
      for line in in_f:
        cols = line.decode('utf-8').rstrip('\n').split('\t')
        start = float(cols[0])
        end = float(cols[1])
        duration = float(cols[2])
        text = cols[3]
        transcribed.append((start, end, duration, text))
  lyrics = []
  with open(lyric_file, 'rb') as in_f:
    for line in in_f:
      pass
  return


def formatTsvAsSrt(video_id: str, dry_run: bool) -> str:
  tsv_file = 'tsvs/{video_id}.tsv'.format(video_id=video_id)
  srt_file = 'subtitles/{video_id}.srt'.format(video_id=video_id)
  if dry_run:
    ok = True
    print('python3 tsv2srt.py {tsv_file} {srt_file}'.format(
      tsv_file=tsv_file,
      srt_file=srt_file
    ))
  else:
    ok = tsv2srt(tsv_file, srt_file)
  if not ok:
    print('Failed to convert tsv to srt')
  return srt_file


def addSrtToVideo(
  video_file: str,
  file_name: str,
  srt_file: str,
  dry_run: bool,
) -> None:
  mysystem = lambda command: mysystem_wrapper(dry_run, command)
  _resp = mysystem([
    'ffmpeg',
    '-i',
    'downloads/{video_file}'.format(video_file=video_file),
    '-i',
    srt_file,
    '-s',
    '720x480',
    # '-c',
    # 'copy',
    '-c:s',
    'mov_text',
    'final/{filename}.mp4'.format(filename=file_name),
  ])
  return


def gen_subtitles(
  url: str,
  file_name: str,
  lang: str,
  lyric_file: Optional[str],
  dry_run: bool,
) -> None:
  aws_bucket = 'subtitler1'
  gcp_bucket = 'subtitler2'
  aws_region = 'us-east-2'

  job_id = urandom5()
  video_id = youtubeVideoID(url)
  print('Running job_id: {job_id}'.format(job_id=job_id))

  if not dry_run:
    with open('video_ids/{video_id}.json'.format(video_id=video_id), 'wb') as f:
      f.write(json.dumps({'job_id': job_id, 'video_name': file_name}, sort_keys=True).encode('utf-8'))
      f.write(b'\n')
    with open('video_names/{filename}.json'.format(filename=file_name), 'wb') as f:
      f.write(json.dumps({'job_id': job_id, 'video_id': video_id}, sort_keys=True).encode('utf-8'))
      f.write(b'\n')

  video_file = downloadVideo(url, video_id, dry_run)
  extractAudio(video_file, video_id, dry_run)
  maybeSpleeter(video_id, dry_run)
  uploadAudioToAws(aws_bucket, video_id, dry_run)
  startAwsTranscriptJob(job_id, lang, aws_bucket, video_id, aws_region, dry_run)
  uploadAudioToGcp(gcp_bucket, video_id, dry_run)
  startGcpTranscriptJob(lang, gcp_bucket, video_id, dry_run)
  # TODO add --model_file and evaluate it here on audios/{video_id}.wav
  waitForAwsTranscriptions(aws_region, dry_run)
  downloadAwsTranscriptions(job_id, aws_region, video_id, dry_run)
  # `output2tsv` calls `normalizeTextContent`, which lowercases and transliterates
  output2tsv(video_id, dry_run)
  if lyric_file is not None:
    alignLyricFile(video_id, lyric_file, dry_run)
  else:
    print('No lyric file', file=sys.stderr)
  # TODO join aws outputs/{video_id}.json, model eval output, and lyrics file
  srt_file = formatTsvAsSrt(video_id, dry_run)
  addSrtToVideo(video_file, file_name, srt_file, dry_run)

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
  * jq
  """
  parser = argparse.ArgumentParser('Download and transcribe a video from youtube')
  parser.add_argument('video_url')
  parser.add_argument('temp_video_file')
  parser.add_argument('language')
  parser.add_argument('lyrics_file', nargs='?', default=None)
  parser.add_argument('--dry-run', action='store_true', help='Only print commands ' +
                      'that would have run')
  args = parser.parse_args()

  language = args.language.lower()
  assert language in ['english', 'hindi', 'hindi-en']
  lang = 'en-US'
  if language == 'hindi':
    lang = 'hi-IN'
  elif language == 'hindi-en':
    lang = 'en-IN'

  gen_subtitles(
    args.video_url,
    args.temp_video_file,
    lang,
    args.lyrics_file,
    args.dry_run
  )
  return
  # end def main


if __name__ == '__main__':
  main()
