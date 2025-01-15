import math # for math.nan!

import matplotlib.pyplot as plt
import numpy as np
from scipy.io.wavfile import read
from scipy.fftpack import dct, idct

window_size_ms = 10 # ms
step_size_ms = 5 # ms

sampleRateOrig, arrOrig = read('../original.wav')
print(f'original sample rate = {sampleRateOrig}, shape = {arrOrig.shape}')

# something is wonky about this test.wav file... scipy.io.wavfile.read keeps complaining about it
sampleRateTest, arrTest = read('../test.wav')
print(f'test sample rate = {sampleRateTest}, shape = {arrTest.shape}')

assert sampleRateOrig == sampleRateTest, 'I expected consistent sample rates'
assert arrTest.shape[0] <= arrOrig.shape[0], 'I expected a longer original wav than test wav'

# these sizes represent the number of audio samples (air pressure)
# divide by 1000 because window_size_ms is in ms, and sampleRateOrig is in sec
window_size = sampleRateOrig * window_size_ms // 1000 
step_size = sampleRateOrig * step_size_ms // 1000
num_windowed_samples = (arrTest.shape[0] - window_size) // step_size 
print(f'window_size = {window_size}, step_size = {step_size}, num_windowed_samples = {num_windowed_samples}')

trunc_orig = arrOrig[:num_windowed_samples * step_size + window_size, 0]
trunc_test = arrTest[:num_windowed_samples * step_size + window_size]
print(f'trunc_orig.shape = {trunc_orig.shape}, trunc_test.shape = {trunc_orig.shape}')

# chunkify was generated with the help of chatgpt
def chunkify(arr, length, step):
    # numpy.ndarray[n,], x, y -> numpy.ndarray[m,x]
    """Given a 1-d numpy array of length n, a window length, and a step size, return
    a 2-d numpy array where each row represents a window over the input array. This
    includes overlap if step size is less than window length."""
    return np.array([arr[i:i+length] for i in range(0, arr.shape[0] - length + 1, step)])

windowed_orig = chunkify(trunc_orig, window_size, step_size)
windowed_test = chunkify(trunc_test, window_size, step_size)
print(f'windowed_orig.shape = {windowed_orig.shape}, windowed_test.shape = {windowed_test.shape}')

def freqs(arr):
    # np.ndarray[n, m] -> (np.ndarray[n,], np.ndarray[n, m])
    """Given a 2-d numpy array with n rows, where each row has m elements, return
    the index of the max element for each row and the DCT (type 2) transformed
    frequencies of the rotated values."""
    # find the index of the max value for each row (axis=1)
    max_i = np.argmax(arr, axis=1)
    rotated = np.copy(arr) 
    for row_i in range(arr.shape[0]):
        # first half of values from max_i[row_i] onwards
        rotated[row_i, 0:(arr.shape[1] - max_i[row_i])] = arr[row_i, max_i[row_i]:]
        # second half of values, up until the max_i element
        rotated[row_i, (arr.shape[1] - max_i[row_i]):] = arr[row_i, 0:max_i[row_i]]
    # invert dct with: `normalized = 1.0 / (2 * arr.shape[1]) * scipy.fftpack.idct(freqs, type=2, norm=None, axis=1)`
    return max_i, dct(rotated, type=2, norm=None, axis=1)

# I'm ignoring the max_i results for now...
_, freqs_orig = freqs(windowed_orig)
_, freqs_test = freqs(windowed_test)
print(f'freqs_orig.shape = {freqs_orig.shape}, freqs_test.shape = {freqs_test.shape}')

# summarize_columns(freqs_orig) to get 441 summaries across ~2k rows
# summarize_columns(freqs_test) to get 441 summaries across ~2k rows

# try cross correlating the columns within each table? find how different frequencies correlate?

# euclidean was generated with the help of chatgpt
def euclidean(arr1, arr2):
    # np.ndarray[n,], np.ndarray[n,] -> float
    return np.linalg.norm(arr1 - arr2)

def cosine(arr1, arr2):
    # np.ndarray[n,], np.ndarray[n,] -> float
    return np.dot(arr1, arr2) / (np.linalg.norm(arr1) * np.linalg.norm(arr2))

# compute ~2k x ~2k comparisons of brute_force(freqs_orig, freqs_test)
comps = np.zeros((freqs_orig.shape[0], freqs_test.shape[0]))
for i in range(freqs_orig.shape[0]):
    for j in range(freqs_test.shape[0]):
        comps[i, j] = euclidean(freqs_orig[i], freqs_test[j])

plt.imshow(comps, cmap='viridis', interpolation='nearest')
plt.colorbar()  # Add a color bar to show the scale
plt.title("Heatmap using Matplotlib")
plt.show()

