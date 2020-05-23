# misc.py
# Trevor Pottinger
# Mon May 18 19:53:44 PDT 2020

import base64
import math
import pickle
import random
from typing import Any, Dict, List, Optional, Tuple

import numpy as np
import pandas as pd
import scipy.io.wavfile
import scipy.signal
import sklearn.tree
import sklearn.ensemble


def urandom5() -> str:
  """Reads 5 bytes from /dev/urandom and encodes them in lowercase base32"""
  with open('/dev/urandom', 'rb') as f:
    return base64.b32encode(f.read(5)).decode('utf-8').lower()


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
  utt_file: Optional[str],
  limit_seconds: float,
) -> Dict[str, Any]:
  # dtype should be np.dtype('int16')
  rate, all_data = scipy.io.wavfile.read(audio_file)
  # TODO allow for sampling strategies other than first-N and first channel
  data = all_data[:int(rate * limit_seconds), 0]

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
  if utt_file is not None:
    # TODO wrap this in a helper method
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
  if utt_file is None:
    is_talking = np.repeat(np.nan, spectro.T.shape[0])
  is_talking = is_talking[:-1]
  was_talking = np.hstack([[0], is_talking[:-1]])
  was_was_talking = np.hstack([[0, 0], is_talking[:-2]])

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
    'is_talking': is_talking,
    'was_talking': was_talking,
    'was_was_talking': was_was_talking,
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
    'was_talking': data['was_talking'],
    'was_was_talking': data['was_was_talking'],
    # TODO phoneme
  })


def utterancesFromPredictions(
  min_word_width: int,
  windows_per_second: float,
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
      # TODO content is TBD!
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


# Copied from notebooks/test-training.ipynb
def trainModel(
  df: pd.DataFrame,
  out_file_name: str,
  max_depth: int = 5,
  num_frequencies: int = 60,
  rand_int: Optional[int] = None,
  num_normalization_buckets: int = 20,
) -> str:
  # Normalize the features
  normalized = normalizeFreqs(
    np.abs(np.asarray(df.freqs_vec.tolist()))[:, :num_frequencies],
    num_normalization_buckets
  )
  combined = np.hstack([
    df.was_was_talking.to_numpy().reshape(normalized.shape[0], 1),
    df.was_talking.to_numpy().reshape(normalized.shape[0], 1),
    normalized,
  ])

  # Train the model
  if rand_int is None:
    rand_int = random.randint(0, 2 ** 32 - 1)
  # all: max_depth=n, max_features=s/n
  # sklearn.ensemble.GradientBoostingClassifier(n_estimators=n, learning_rate=0-1)
  # sklearn.ensemble.GradientBoostingRegressor(n_estimators=n, learning_rate=0-1)
  # `from sklearn.experimental import enable_hist_gradient_boosting`
  # sklearn.ensemble.HistGradientBoostingClassifier(max_iter=n, learning_rate=0-1)
  # sklearn.ensemble.HistGradientBoostingRegressor(max_iter=n, learning_rate=0-1)
  # sklearn.ensemble.RandomForestClassifier(n_estimators=n)
  # sklearn.ensemble.RandomForestRegressor(n_estimators=n)
  # sklearn.tree.DecisionTreeClassifier()
  # sklearn.tree.DecisionTreeRegressor()
  classifier = sklearn.tree.DecisionTreeClassifier(
    random_state=rand_int,
    max_depth=max_depth
  )
  model = classifier.fit(combined, df.is_talking)

  with open(out_file_name, 'wb') as model_file:
    pickle.dump(model, model_file)
  return out_file_name
  # end def trainModel


def evalModel(
  model_file: str,
  audio_files: List[str],
  tsv_files: List[Optional[str]],
  num_frequencies: int = 60,
  limit_seconds: float = 60.0,
  num_normalization_buckets: int = 20,
) -> Tuple[pd.DataFrame, np.ndarray]:
  read_func = lambda aud, tsv: dict2packed(readData(aud, tsv, limit_seconds))
  eval_df = pd.concat([read_func(*files) for files in zip(audio_files, tsv_files)])
  eval_normalized = normalizeFreqs(
    np.abs(np.asarray(eval_df.freqs_vec.tolist()))[:, :num_frequencies],
    num_normalization_buckets
  )

  with open(model_file, 'rb') as model_f:
    # TODO mmap read the model file to avoid the IO hit
    # Also, note that pickle.load may run arbitrary python, so don't run
    # models from people you don't know.
    model = pickle.load(model_f)

  predictions = np.zeros(eval_normalized.shape[0], dtype='int8')
  for i in range(eval_normalized.shape[0]):
    if i < 2:
      predictions[i] = 0
      continue
    predictions[i] = model.predict([np.hstack([
      predictions[i - 2],
      predictions[i - 1],
      eval_normalized[i],
    ])])

  # Any prediction shorter than 8 frames will be considered noise. Assuming
  # each frame is 10 ms and step size is 5 ms, then the shortest word can
  # be is 45 ms.
  min_word_width = 8
  predicted_utterances = utterancesFromPredictions(
    min_word_width,
    eval_df.signal_rate.iat[0] / eval_df.step_size.iat[0],
    predictions
  )
  print('start\tend\tduration\tcontent')
  for utt in predicted_utterances:
    print('\t'.join([str(val) for val in [
      utt['start'],
      utt['end'],
      utt['duration'],
      '',  # TODO content is TBD!
    ]]))
  return eval_df, predictions
  # end def evalModel


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
