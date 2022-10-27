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
from typing import (Any, Dict, IO, List, Optional, Tuple)
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


def mysystem_wrapper(
  dry_run: bool,
  command: List[str],
  output: Optional[IO] = None,
) -> int:
  if dry_run:
    print(" ".join(command))
    return 0
  res = subprocess.run(command, stdout=output)
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
    'python3',
    '-m',
    'youtube_dl',
    '--no-cache-dir',
    '--no-playlist',
    '--all-subs',
    url,
    '--output',
    'data/downloads/{video_id}.%(ext)s'.format(
      video_id=video_id
    ),
  ])
  files = [f for f in os.listdir('data/downloads') if f.startswith(video_id)]
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
    '-n',
    '-i',
    'data/downloads/{video_file}'.format(video_file=video_file),
    'data/audios/{video_id}.wav'.format(video_id=video_id),
  ])
  return


def maybeSpleeter(video_id: str, dry_run: bool) -> None:
  global spleeter
  if spleeter is None:
    print('spleeter is not installed', file=sys.stderr)
  mysystem = lambda command: mysystem_wrapper(dry_run, command)
  resp = mysystem([
    'python3',
    '-m',
    'spleeter',
    'separate',
    '-p',
    'spleeter:2stems',
    '-o',
    'data/audios/',
    'data/audios/{video_id}.wav'.format(video_id=video_id),
  ])
  if resp != 0:
    print('spleeter failed to run. Return code = %d' % resp, file=sys.stderr)
    spleeter = None
    return
  _resp = mysystem([
    'ffmpeg',
    '-n',
    '-i',
    'data/audios/{video_id}/vocals.wav'.format(video_id=video_id),
    '-map_channel',
    '0.0.0',
    'data/audios/{video_id}/vocals_left.wav'.format(video_id=video_id),
    '-map_channel',
    '0.0.1',
    'data/audios/{video_id}/vocals_right.wav'.format(video_id=video_id),
  ])
  return


def uploadAudioToAws(bucket: str, video_id: str, dry_run: bool) -> None:
  mysystem = lambda command: mysystem_wrapper(dry_run, command)
  # Result should be https://s3.console.aws.amazon.com/s3/buckets/subtitler1/?region=us-east-2
  audio_format = 'data/audios/{video_id}.wav'
  if spleeter is not None:
    # TODO figure out how to pass multi channel to GCP
    audio_format = 'data/audios/{video_id}/vocals_left.wav'
  audio_file = audio_format.format(video_id=video_id)
  _resp = mysystem([
    'python3',
    '-m',
    'awscli',
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
    'python3',
    '-m',
    'awscli',
    'transcribe',
    'start-transcription-job',
    '--cli-input-json',
    'file://job-start-command.json',
    '--region',
    region,
  ])
  return
  # end def startAwsTranscriptJob


def waitForAwsTranscriptions(region: str, dry_run: bool) -> None:
  for _ in range(200):
    # Note: `--status IN_PROGRESS` is optional
    res = mysystem2(dry_run, [
      'python3',
      '-m',
      'awscli',
      'transcribe',
      'list-transcription-jobs',
      '--status',
      'IN_PROGRESS',
      '--region',
      region,
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
  command = 'curl -o data/outputs/aws_{video_id}.json "$(python3 -m awscli transcribe get-transcription-job --region {region} --transcription-job-name {job_id} | jq -r .TranscriptionJob.Transcript.TranscriptFileUri)"'.format(
    job_id=job_id,
    region=region,
    video_id=video_id
  )
  if dry_run:
    print(command)
  else:
    _resp = os.system(command)
  return


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
  command = 'jq -cr \'.results.items[] | select(.start_time != null) | [.start_time, ((.end_time | tonumber) - (.start_time | tonumber)), (.alternatives[0].content | ascii_downcase)] | @tsv\' data/outputs/aws_{video_id}.json > data/tsvs/aws_{video_id}.tsv'.format(
    video_id=video_id
  )
  if not dry_run:
    results = []
    with open('data/outputs/aws_{video_id}.json'.format(video_id=video_id), 'rb') as in_f:
      j_obj = json.loads(in_f.read().decode('utf-8'))
      for item in j_obj['results']['items']:
        if 'start_time' not in item:
          continue
        results.append((
          item['start_time'],
          float(item['end_time']) - float(item['start_time']),
          item['alternatives'][0]['content'].lower()
        ))
    with open('data/tsvs/aws_{video_id}.tsv'.format(video_id=video_id), 'wb') as out_f:
      for item in results:
        out_f.write('{start}\t{duration:0.3f}\t{content}\n'.format(
          start=item[0],
          duration=item[1],
          content=normalizeTextContent(item[2])
        ).encode('utf-8'))
  else:
    print(command)
    _resp = os.system(command)
  return
  # end def output2tsv


def uploadAudioToGcp(bucket: str, video_id: str, dry_run: bool) -> None:
  mysystem = lambda command: mysystem_wrapper(dry_run, command)
  # Result should be https://console.cloud.google.com/storage/browser/subtitler2?forceOnBucketsSortingFiltering=false&authuser=1&project=emerald-cacao-282303
  audio_format = 'data/audios/{video_id}.wav'
  if spleeter is not None:
    # TODO figure out how to pass multi channel to GCP
    audio_format = 'data/audios/{video_id}/vocals_left.wav'
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
) -> Optional[str]:
  mysystem = lambda command: mysystem2(dry_run, command)
  command = [
    'gcloud',
    'ml',
    'speech',
    'recognize-long-running',
    '--include-word-time-offsets',
    '--async',
    '--language-code={lang}'.format(lang=lang),
    'gs://{bucket}/{video_id}.wav'.format(bucket=bucket, video_id=video_id),
  ]
  print(" ".join(command))
  resp = mysystem(command)
  if resp is None:
    print('empty gcloud response!', file=sys.stderr)
    return None
  print('gcloud response!')
  print(resp)
  return json.loads(resp)['name']
  # end def startGcpTranscriptJob


def waitForGcpTranscriptions(
  video_id: str,
  job_id: Optional[str],
  dry_run: bool,
) -> Optional[str]:
  if job_id is None:
    dry_run = True
  mysystem = lambda command: mysystem2(dry_run, command)
  command = [
    'gcloud',
    'ml',
    'speech',
    'operations',
    'wait',
    job_id if job_id is not None else 'null_job_id',
  ]
  print(" ".join(command))
  resp = mysystem(command)
  if resp is None:
    print('No transcription from google!', file=sys.stderr)
    return None
  print('gcloud response!')
  print(resp)
  # similar to output2tsv
  with open('data/outputs/gcp_{video_id}.tsv'.format(video_id=video_id), 'wb') as f:
    f.write(resp.encode('utf-8'))
  obj = json.loads(resp)
  with open('data/tsvs/gcp_{video_id}.tsv'.format(video_id=video_id), 'wb') as f:
    # Kinda like `jq '.results[] | .alternatives[] | .words'`
    for thing in obj['results']:
      for other in thing['alternatives']:
        for word in other['words']:
          # [:-1] cause google appends an "s" to all their times
          f.write(("\t".join([
            word['startTime'][:-1],
            '%.3f' % (float(word['endTime'][:-1]) - float(word['startTime'][:-1])),
            word['word'],
          ])).encode('utf-8'))
          f.write(b'\n')
  return None
  # end def waitForGcpTranscriptions


def alignLyricFile(video_id: str, lyric_file: str, dry_run: bool) -> None:
  if dry_run:
    print('python3 align_lyrics.py data/tsvs/aws_{video_id}.tsv {lyric_file}'.format(
      lyric_file=lyric_file,
      video_id=video_id
    ))
    return
  if not os.path.isfile(lyric_file):
    print('Lyrics for {lyric_file} don\'t exist'.format(lyric_file=lyric_file), file=sys.stderr)
    return
  # TODO call alignLyrics('data/tsvs/aws_{video_id}.tsv'.format(video_id=video_id), lyric_file)
  transcribed = []
  with open('data/tsvs/aws_{video_id}.tsv'.format(video_id=video_id), 'rb') as in_f:
      for line in in_f:
        cols = line.decode('utf-8').rstrip('\n').split('\t')
        start = float(cols[0])
        duration = float(cols[1])
        text = cols[2]
        transcribed.append((start, duration, text))
  lyrics = []
  with open(lyric_file, 'rb') as in_f:
    for line in in_f:
      pass
  return


def formatTsvAsSrt(video_id: str, dry_run: bool) -> str:
  tsv_file = 'data/tsvs/aws_{video_id}.tsv'.format(video_id=video_id)
  srt_file = 'data/subtitles/{video_id}.srt'.format(video_id=video_id)
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
    '-n',
    '-i',
    'data/downloads/{video_file}'.format(video_file=video_file),
    '-i',
    srt_file,
    '-s',
    '720x480',
    '-c:s',
    'mov_text',
    'data/final/{filename}.mp4'.format(filename=file_name),
  ])
  return


def evalModel(video_id: str, model_file: str, dry_run: bool) -> None:
  audio_format = 'data/audios/{video_id}.wav'
  if spleeter is not None:
    # TODO figure out how to pass multi channel to GCP
    audio_format = 'data/audios/{video_id}/vocals_left.wav'
  out_file = 'data/tsvs/predicted_{video_id}.tsv'.format(video_id=video_id)
  with open(out_file, 'wb') as f:
    _resp = mysystem_wrapper(
      dry_run,
      [
        'python3',
        'eval.py',
        model_file,
        video_id,
      ],
      f
    )
  return


def gen_subtitles(
  url: str,
  file_name: str,
  lang: str,
  lyric_file: Optional[str],
  model_file: Optional[str],
  aws_bucket: Optional[str],
  aws_region: Optional[str],
  gcp_bucket: Optional[str],
  dry_run: bool,
) -> None:
  job_id = urandom5()
  video_id = youtubeVideoID(url)
  print('Running job_id: {job_id}'.format(job_id=job_id))

  if not dry_run:
    with open('data/video_ids/{video_id}.json'.format(video_id=video_id), 'wb') as f:
      f.write(json.dumps({'job_id': job_id, 'video_name': file_name}, sort_keys=True).encode('utf-8'))
      f.write(b'\n')
    with open('data/video_names/{filename}.json'.format(filename=file_name), 'wb') as f:
      f.write(json.dumps({'job_id': job_id, 'video_id': video_id}, sort_keys=True).encode('utf-8'))
      f.write(b'\n')

  video_file = downloadVideo(url, video_id, dry_run)
  extractAudio(video_file, video_id, dry_run)
  maybeSpleeter(video_id, dry_run)
  if aws_bucket is not None:
    assert aws_region is not None, 'aws_bucket specified without aws_region'
    uploadAudioToAws(aws_bucket, video_id, dry_run)
    startAwsTranscriptJob(job_id, lang, aws_bucket, video_id, aws_region, dry_run)
  if gcp_bucket is not None:
    uploadAudioToGcp(gcp_bucket, video_id, dry_run)
    gcp_job_id = startGcpTranscriptJob(lang, gcp_bucket, video_id, dry_run)

  if model_file is not None:
    evalModel(video_id, model_file, dry_run)
  if aws_bucket is not None:
    waitForAwsTranscriptions(aws_region, dry_run)
    downloadAwsTranscriptions(job_id, aws_region, video_id, dry_run)
  # `output2tsv` calls `normalizeTextContent`, which lowercases and transliterates
  output2tsv(video_id, dry_run)
  if gcp_bucket is not None:
    waitForGcpTranscriptions(video_id, gcp_job_id, dry_run)

  if lyric_file is not None:
    alignLyricFile(video_id, lyric_file, dry_run)
  else:
    print('No lyric file', file=sys.stderr)

  # TODO join aws data/outputs/aws_{video_id}.json, model eval output, and lyrics file
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
  parser.add_argument('--model', help='The model file to pass to eval.py')
  parser.add_argument('--aws_bucket', help='The name of the AWS S3 bucket to use')
  parser.add_argument('--aws_region', help='The name of the AWS region to use')
  parser.add_argument('--gcp_bucket', help='The name of the GCP storage bucket to use')
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
    args.model,
    args.aws_bucket,
    args.aws_region,
    args.gcp_bucket,
    args.dry_run
  )
  return
  # end def main


if __name__ == '__main__':
  main()
