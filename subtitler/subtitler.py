# subtitler.py
# Trevor Pottinger
# Tue Apr 28 19:50:37 PDT 2020

import argparse
import json
import os


def mysystem(command: str) -> None:
  if False:
    os.system(command)
  else:
    print(command)


def run(url: str, filename: str, lang: str) -> None:
  bucket = 'subtitler1'
  region = 'us-east-2'
  job_name = 'test-job-1'  # TODO randomize

  _resp = mysystem('youtube-dl {url} --output "temp.%(ext)s"'.format(url=url))
  _resp = mysystem('mv temp.* {filename}'.format(filename=filename))
  # supported file types: mp3 | mp4 | wav | flac
  # from https://docs.aws.amazon.com/transcribe/latest/dg/API_TranscriptionJob.html#transcribe-Type-TranscriptionJob-MediaFormat
  _resp = mysystem('ffmpeg -i {filename} temp.wav'.format(filename=filename))

  # Result should be https://s3.console.aws.amazon.com/s3/buckets/subtitler1/?region=us-east-2
  _resp = mysystem('aws s3 cp temp.wav s3://{bucket}/'.format(bucket=bucket))
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
  _resp = mysystem('echo \'{json_job_str}\' > job-start-command.json'.format(json_job_str=json_job_str))
  _resp = mysystem('aws transcribe start-transcription-job --region {region} --cli-input-json file://job-start-command.json'.format(region=region))

  # TODO do while in-progress
  # Then `aws transcribe list-transcription-jobs --region us-east-2 [--status IN_PROGRESS]`
  _resp = mysystem('curl -o output.json "$(aws transcribe get-transcription-job --region {region} --transcription-job-name {job_name} | jq -r .TranscriptionJob.Transcript.TranscriptFileUri)"'.format(job_name=job_name, region=region))

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
  return


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
