# misc.py
# Trevor Pottinger
# Mon May 18 19:53:44 PDT 2020

import math
from typing import Any, Dict, List, Tuple

import numpy as np
import pandas as pd
import scipy.io.wavfile
import scipy.signal


def labelsFromUtterances(
	utterances: List[Dict[str, Any]],
	windows_per_second: int,
	n_rows: int,
) -> Tuple[int, np.ndarray]:
  """return: np.ndarray[ndtype=intish, shape=[n_rows]]"""
  positive_examples = 0
  is_talking = np.zeros(n_rows, dtype=np.int8)
  for item in utterances:
    # math.ceil rounds up to the latest millisecond for labeling
    start_i = int(math.ceil(item['start'] * windows_per_second))
    end_i = int(math.ceil(item['end'] * windows_per_second))
    for i in range(start_i, min(end_i, n_rows)):
      is_talking[i] = 1
      positive_examples += 1
  return positive_examples, is_talking


# TODO use a dataclass instead of a Dict
def readData(
  audio_file: str,
  utt_file: str,
  limit_seconds: int,
) -> Dict[str, Any]:
  # dtype should be np.dtype('int16')
  rate, all_data = scipy.io.wavfile.read(audio_file)
  # TODO allow for sampling strategies other than first-N and first channel
  data = all_data[:rate * limit_seconds, 0]

  # try wrapping in `int(2 ** math.ceil(math.log(.., 2)))`
  window_size = int(rate) // 100
  step_size = window_size // 2
  # we want windows_per_second to be 200
  windows_per_second = int(rate) // step_size
  _freqs, _times, spectro = scipy.signal.stft(
    data,
    rate,
    window='hann', # default, as specified by the documentation
    nperseg=window_size,
    noverlap=window_size // 2
  )

  utterances = []
  with open(utt_file, 'rb') as f:
    for line in f:
      cols = [s.decode('utf-8') for s in line.rstrip(b'\n').split(b'\t')]
      utterances.append({
        'start': float(cols[0]),
        'end': float(cols[1]),
        'duration': float(cols[2]),
        'content': cols[3],
      })

  _num_examples, is_talking = labelsFromUtterances(
    utterances, 
    windows_per_second, 
    spectro.T.shape[0]
  )

  # TODO understand stft input and output shapes
  # Drop the last frame because I don't know how it's derived...
  return {
    'file_name': audio_file,
		'label_file_name': utt_file,
    'signal_rate': rate,
    'window_size': window_size,
    'step_size': step_size,
    'num_utterances': len(utterances),
    'data': data, # TODO remove this line
    'freqs_vec': spectro.T[:-1, :-1],
    'is_talking': is_talking[:-1],
    # TODO phoneme
  }


def dict2packed(data: Dict[str, Any]) -> pd.DataFrame:
  num_rows = data['freqs_vec'].shape[0]
  step_size = data['step_size']  # type: int
  window_size = data['window_size']  # type: int
  frames = []
  for i in range(0, data['data'].shape[0], step_size):
    # Cast to lists because pandas doesn't allow numpy.ndarray in cells
    frames.append(list(data['data'][i : i + window_size]))
  return pd.DataFrame(data={
    'file_name': [data['file_name']] * num_rows,
		'label_file_name': [data['label_file_name']] * num_rows,
    'signal_rate': [data['signal_rate']] * num_rows,
    'window_size': [window_size] * num_rows,
    'step_size': [step_size] * num_rows,
    'num_utterances': [data['num_utterances']] * num_rows,
    'raw_signal_vec': frames,
    'freqs_vec': data['freqs_vec'].tolist(),
    'is_talking': data['is_talking'],
    # TODO phoneme
  })


def utterancesFromPredictions(
  min_word_width: int,
  windows_per_second: int,
  predictions: np.ndarray,
) -> List[Dict[str, Any]]:
  ones = np.ones(min_word_width)
  i = 0
  predicted_utterances = []
  while i < predictions.shape[0] - min_word_width + 1:
    # 0 means ~"no words are spoken"
    if predictions[i] == 0:
      i += 1
      continue
    if (predictions[i : i + min_word_width] != ones).all():
      predictions[i] = 0
      i += 1
      continue
    for j in range(i + min_word_width, predictions.shape[0]):
      if predictions[j] == 0:
        j -= 1
        break
    predicted_utterances.append({
      'start': i / windows_per_second,
      'end': j / windows_per_second,
      'duration': (j - i) / windows_per_second,
      # content is TBD!
    })
    i = j + 1
  return predicted_utterances


def normalizeFreqs(freqs: np.ndarray, n_buckets: int) -> np.ndarray:
  # TODO scale dtype according to n_buckets
  assert n_buckets < 250, 'limit n_buckets to fit in int8'
  ret = np.zeros(freqs.shape, dtype=np.int8)
  buckets = list(map(lambda n: n / n_buckets, range(1, n_buckets)))
  # Take the transpose so each row represents the quantile summaries for
  # each column in freqs
  quantiles = np.quantile(freqs, buckets, axis=0).T
  # Enumerate each column of input
  # TODO vectorize this...
  for i in range(freqs.shape[1]):
    ret[:, i] = np.searchsorted(quantiles[i], freqs[:, i])
  return ret


def corrTwoDF(data: pd.DataFrame, a: str, b: str) -> None:
  # TODO include helpful error messages here
  assert a in data.columns
  assert b in data.columns
  # TODO assert that data.dtypes.a is categorical-ish
  # TODO assert that data.dtypes.b is categorical-ish
  return


def corrTwo(a: pd.Series, b: pd.Series) -> None:
  data = pd.DataFrame([a, b])
  print(data.groupby(['a', 'b']).size())
  return None
