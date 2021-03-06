# misc.py
# Trevor Pottinger
# Mon May 18 19:53:44 PDT 2020

import base64
import math
import pickle
import random
import sys
import time
from typing import Any, Dict, List, Optional, Sequence, Tuple

import numpy as np
import pandas as pd
import scipy.io.wavfile
import scipy.signal
import scipy.stats
from sklearn.experimental import enable_hist_gradient_boosting
import sklearn.ensemble
import sklearn.metrics
import sklearn.tree


MIN_LENGTH = 0.000001


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
  limit_seconds: Optional[float],
) -> Dict[str, Any]:
  # dtype should be np.dtype('int16')
  rate, all_data = scipy.io.wavfile.read(audio_file)

  # TODO allow for sampling strategies other than first-N and first channel
  if limit_seconds is not None:
    data = all_data[:int(rate * limit_seconds)]
  else:
    data = all_data[:]

  if len(data.shape) > 1 and data.shape[1] > 1:
    data = data[:, 0]

  # try wrapping in `int(2 ** math.ceil(math.log(.., 2)))`
  window_size = int(rate) // 100
  step_size = window_size // 2
  # we want windows_per_second to be 200
  windows_per_second = int(rate) // step_size
  expected_num_frames = data.shape[0] // step_size

  _freqs, _times, spectro = scipy.signal.stft(
    data,
    rate,
    window='hann', # default, as specified by the documentation
    nperseg=window_size,
    noverlap=window_size // 2
  )

  # TODO why is this step_size and not window_size? stft's output shape is weird
  if spectro.shape[0] > step_size:
    spectro = spectro[:step_size]
  if spectro.shape[1] > expected_num_frames:
    spectro = spectro[:, :expected_num_frames]
  if spectro.shape[1] < expected_num_frames:
    data = data[:spectro.shape[1] * step_size]

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
    'freqs_vec': spectro.T,
    'is_talking': is_talking,
    'was_talking': was_talking,
    'was_was_talking': was_was_talking,
    # TODO(7d79) phoneme
  }


def dict2packed(data: Dict[str, Any]) -> pd.DataFrame:
  num_rows = data['freqs_vec'].shape[0]
  step_size = data['step_size']  # type: int
  window_size = data['window_size']  # type: int
  frames = []
  for i in range(0, data['data'].shape[0], step_size):
    # Cast to lists because pandas doesn't allow numpy.ndarray in cells
    frames.append(list(data['data'][i : i + window_size]))
  # TODO distances, unit_lengths, angles
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
    # TODO(7d79) phoneme
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
def trainScorer(
  df: pd.DataFrame,
  out_file_name: str,
  n_iter: int = 10,
  num_frequencies: int = 60,
  rand_int: Optional[int] = None,
  num_normalization_buckets: int = 20,
) -> str:
  # TODO(177c) also output the quantiles that are derived in normalization
  # Normalize the features
  normalized = np.abs(np.asarray(df.freqs_vec.tolist()))[:, :num_frequencies]
  # normalized = normalizeFreqs(
    # np.abs(np.asarray(df.freqs_vec.tolist()))[:, :num_frequencies],
    # num_normalization_buckets
  # )

  distances = np.zeros(df.shape[0])
  distances[1:] = np.sqrt(np.sum(
    np.square(normalized[:-1] - normalized[1:]),
    axis=1
  ))

  # This computation of angles has the potential to to have a floating point
  # error. As in, `np.sum(norm[1:] * norm[:-1]) / (lens[1:] * lens[:-1])` can
  # ever so slightly exceed 1.0.
  unit_lengths = np.sqrt(np.sum(np.square(normalized), axis=1))
  unit_lengths[unit_lengths < MIN_LENGTH] = MIN_LENGTH
  angles = np.zeros(df.shape[0])
  numerators = np.sum(normalized[1:] * normalized[:-1], axis=1)
  denominators = unit_lengths[1:] * unit_lengths[:-1]
  # This avoids np.arccos resulting in np.nan
  numerators[numerators > denominators] = denominators[numerators > denominators]
  angles[1:] = np.arccos(numerators / denominators)

  # Adding was_talking, and was_was_talking, is too hard to get right. We would need
  # a RNN or LSTM model that can train with several qualities of was_talking. Otherwise,
  # we train with a high quality signal but eval with a less idea, production signal.
  combined = np.hstack([
    distances.reshape(-1, 1),
    angles.reshape(-1, 1),
    unit_lengths.reshape(-1, 1),
    normalized,
  ])

  if rand_int is None:
    rand_int = random.randint(0, 2 ** 32 - 1)
  print('Seeding random state with %d' % rand_int, file=sys.stderr)

  # Train the model
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

  search = sklearn.model_selection.RandomizedSearchCV(
    sklearn.ensemble.HistGradientBoostingClassifier(),
    {
      # 'l2_regularization': [0.0, 0.5, 1.0, 1.5],
      'learning_rate': scipy.stats.uniform(loc=1e-4, scale=1.0 - 2e-4),
      'max_bins': scipy.stats.randint(low=4, high=128),
      'max_depth': scipy.stats.randint(low=4, high=12),
      # 'max_features': scipy.stats.randint(low=10, high=20),
      # 'n_estimators': scipy.stats.randint(low=10, high=100),
      'max_iter': scipy.stats.randint(low=50, high=150),
      # 'n_jobs': [-1],
    },
    n_iter=n_iter,
    random_state=rand_int,
    scoring=sklearn.metrics.make_scorer(sklearn.metrics.matthews_corrcoef),
  )
  results = search.fit(combined, df.is_talking)
  print('best_score_ =', results.best_score_)
  print('best_params_ =', results.best_params_)

  with open(out_file_name, 'wb') as model_file:
    pickle.dump(results.best_estimator_, model_file)
  return out_file_name
  # end def trainScorer


def evalScorer(
  model_file: str,
  audio_files: List[str],
  tsv_files: Sequence[Optional[str]],
  num_frequencies: int = 60,
  limit_seconds: float = 600.0,
  num_normalization_buckets: int = 20,
) -> Tuple[pd.DataFrame, np.ndarray]:
  read_func = lambda aud, tsv: dict2packed(readData(aud, tsv, limit_seconds))
  eval_df = pd.concat([read_func(*files) for files in zip(audio_files, tsv_files)])
  # TODO(177c) use the quantiles that are derived in normalization
  eval_normalized = np.abs(np.asarray(eval_df.freqs_vec.tolist()))[:, :num_frequencies]
  # eval_normalized = normalizeFreqs(
    # np.abs(np.asarray(eval_df.freqs_vec.tolist()))[:, :num_frequencies],
    # num_normalization_buckets
  # )

  with open(model_file, 'rb') as model_f:
    # TODO mmap read the model file to avoid the IO hit
    # Also, note that pickle.load may run arbitrary python, so don't run
    # models from people you don't know.
    model = pickle.load(model_f)

  distances = np.zeros(eval_df.shape[0])
  distances[1:] = np.sqrt(np.sum(
    np.square(eval_normalized[:-1] - eval_normalized[1:]),
    axis=1
  ))

  # This computation of angles has the potential to to have a floating point
  # error. As in, `np.sum(norm[1:] * norm[:-1]) / (lens[1:] * lens[:-1])` can
  # ever so slightly exceed 1.0.
  unit_lengths = np.sqrt(np.sum(np.square(eval_normalized), axis=1))
  unit_lengths[unit_lengths < MIN_LENGTH] = MIN_LENGTH
  angles = np.zeros(eval_df.shape[0])
  numerators = np.sum(eval_normalized[1:] * eval_normalized[:-1], axis=1)
  denominators = unit_lengths[1:] * unit_lengths[:-1]
  # This avoids np.arccos resulting in np.nan
  numerators[numerators > denominators] = denominators[numerators > denominators]
  angles[1:] = np.arccos(numerators / denominators)

  # If training included was_talking, then we would need to include previous
  # predictions here.
  scores = model.predict_proba(np.hstack([
    distances.reshape(-1, 1),
    angles.reshape(-1, 1),
    unit_lengths.reshape(-1, 1),
    eval_normalized,
  ]))[:, 1]  # always take the probability of the second class

  return eval_df, scores
  # end def evalScorer


def trainModel(
  df: pd.DataFrame,
  out_file_name: str,
  out_file_name2: str,
  audio_files: List[str],
  label_files: List[str],
  n_iter: int = 10,
  num_frequencies: int = 60,
  limit_seconds: float = 600.0,
  rand_int: Optional[int] = None,
  num_normalization_buckets: int = 20,
) -> Tuple[str, str]:
  start = time.time()
  print('Starting training at %f' % start, file=sys.stderr)
  scorer_file = trainScorer(
    df,
    out_file_name,
    n_iter,
    num_frequencies,
    rand_int,
    num_normalization_buckets,
  )

  # TODO don't read the audio files twice
  eval_df, scores = evalScorer(
    scorer_file,
    audio_files,
    label_files,
    num_frequencies,
    limit_seconds,
    num_normalization_buckets
  )

  utterances = []
  for utt_file in label_files:
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
  # TODO transform utterances to aws predictions

  prev_scores = np.zeros(scores.shape[0])
  prev_prev_scores = np.zeros(scores.shape[0])
  prev_scores[1:] = scores[:-1]
  prev_prev_scores[2:] = scores[:-2]
  combined = np.hstack([
    scores.reshape(-1, 1),
    prev_scores.reshape(-1, 1),
    prev_prev_scores.reshape(-1, 1),
  ])

  search = sklearn.model_selection.RandomizedSearchCV(
    sklearn.ensemble.HistGradientBoostingClassifier(),
    {
      # 'l2_regularization': [0.0, 0.5, 1.0, 1.5],
      'learning_rate': scipy.stats.uniform(loc=1e-4, scale=1.0 - 2e-4),
      'max_bins': scipy.stats.randint(low=4, high=128),
      'max_depth': scipy.stats.randint(low=4, high=12),
      # 'max_features': scipy.stats.randint(low=10, high=20),
      # 'n_estimators': scipy.stats.randint(low=10, high=100),
      'max_iter': scipy.stats.randint(low=50, high=150),
      # 'n_jobs': [-1],
    },
    n_iter=n_iter,
    random_state=rand_int,
    scoring=sklearn.metrics.make_scorer(sklearn.metrics.matthews_corrcoef),
  )
  results = search.fit(combined, df.is_talking)
  print('best_score_ =', results.best_score_)
  print('best_params_ =', results.best_params_)

  now = time.time()
  print('Ending training at %f, elapsed %f' % (now, now - start), file=sys.stderr)
  with open(out_file_name2, 'wb') as model_file:
    pickle.dump(results.best_estimator_, model_file)
  return out_file_name, out_file_name2
  # end def trainModel


def evalModel(
  scorer_file: str,
  model_file: str,
  audio_files: List[str],
  aws_files: List[str],
  label_files: Sequence[Optional[str]],
  num_frequencies: int = 60,
  limit_seconds: float = 600.0,
  num_normalization_buckets: int = 20,
) -> Tuple[pd.DataFrame, np.ndarray, np.ndarray]:
  with open(model_file, 'rb') as model_f:
    # TODO mmap read the model file to avoid the IO hit
    # Also, note that pickle.load may run arbitrary python, so don't run
    # models from people you don't know.
    model = pickle.load(model_f)

  eval_df, scores = evalScorer(
    scorer_file,
    audio_files,
    aws_files,
    num_frequencies,
    limit_seconds,
    num_normalization_buckets
  )

  utterances = []
  for utt_file in aws_files:
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
  # TODO transform utterances to aws predictions

  prev_scores = np.zeros(scores.shape[0])
  prev_prev_scores = np.zeros(scores.shape[0])
  prev_scores[1:] = scores[:-1]
  prev_prev_scores[2:] = scores[:-2]

  predictions = model.predict(np.hstack([
    scores.reshape(-1, 1),
    prev_scores.reshape(-1, 1),
    prev_prev_scores.reshape(-1, 1),
  ]))

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
      round(utt['start'], 3),
      round(utt['end'], 3),
      round(utt['duration'], 3),
      '',  # TODO content is TBD!
    ]]))

  return eval_df, scores, predictions
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


def corrOne(
  df: pd.DataFrame,
  num_normalization_buckets: int = 20,
) -> Tuple[pd.DataFrame, List[pd.Series], List[float]]:
  normalized = normalizeFreqs(
    np.abs(np.asarray(df.freqs_vec.tolist()))[:, :30], # TODO remove [:, :30]
    num_normalization_buckets
  )
  num_rows = normalized.shape[0]
  combined = np.hstack([
    df.is_talking.to_numpy().reshape(-1, 1),
    df.was_talking.to_numpy().reshape(-1, 1),
    df.was_was_talking.to_numpy().reshape(-1, 1),
    normalized,
  ])
  columns = ['is_talking', 'was_talking', 'was_was_talking']
  for i in range(normalized.shape[1]):
    columns.append('norm[{i}]'.format(i=i))
  df = pd.DataFrame(dtype=np.int8, columns=columns, data=combined)
  # TODO call a helper method to generalize this
  summaries = []
  for col in columns:
    summaries.append(df.groupby([col]).size())
  correlations = []
  for i, col in enumerate(columns):
    observed = df.groupby(['is_talking', col]).size()
    diffs = 0.0
    # summaries[0] is the summary for is_talking
    for j in summaries[0].index:
      for k in summaries[i].index:
        obs = observed[(j, k)].item() if (j, k) in observed else 0
        expected = summaries[0][j].item() * summaries[i][k].item() / num_rows
        diffs += (obs - expected) * (obs - expected)
    correlations.append(diffs)
  return df, summaries, correlations
