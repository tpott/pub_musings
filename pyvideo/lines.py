# lines.py
# Trevor Pottinger
# Sun Mar 16 23:09:39 PDT 2014

import math
import sys

import numpy as np
from scipy.signal import convolve2d as conv2d
from scipy.ndimage.filters import convolve as convnd
from scipy.misc import imread, imsave

# imports copyvid ?
# use:
#   vidread
#   vidsave
#   grayFilter
#   add_dimension
from togray import *

def inflate(vid, length=3):
  larger = np.zeros(list(vid.shape) + [length])
  for t in tqdm(range(vid.shape[0]), desc="Inflating grayscale to match rgb"):
    np.copyto(larger[t], add_dimension(vid[t], length=length))
  return larger

def squaredSum(*args):
  sum = lambda l: reduce(lambda a, b: a + b, l)
  square = lambda n: n * n
  return math.sqrt(sum(map(square, list(args))))

def hvLines(arr):
  """Expects arr to be time x height x width. Returns a sobel
  filtered array of the input"""
  lines = arr.copy()
  xx, yy = np.meshgrid((1, -1), (1, -1))
  for t in tqdm(range(arr.shape[0])):
    dx = conv2d(arr[t], xx, mode='same')
    dy = conv2d(arr[t], yy, mode='same')
    for h in range(arr.shape[1]):
      for w in range(arr.shape[2]):
        lines[t, h, w] = squaredSum(dx[h,w], dy[h,w])
  return lines 

def timeAndSpace(arr):
  """Expects arr to be time x height x width. Returns a sobel
  filtered array of the input. Uses numpy ndimage convolve"""
  lines = arr.copy()
  #xx, yy = np.meshgrid((1, -1), (1, -1))
  xx = np.array([[[1, 1], [-1, -1]],[[0, 0], [0, 0]]])
  yy = np.array([[[1, -1], [1, -1]],[[0, 0], [0, 0]]])
  zz = np.array([[[1, 1], [1, 1]],[[-1, -1], [-1, -1]]])
  for t in tqdm(range(arr.shape[0]), desc="Time and space filter"):
    dx = convnd(arr[t:t+2], xx, mode='reflect')
    dy = convnd(arr[t:t+2], yy, mode='reflect')
    dz = convnd(arr[t:t+2], zz, mode='reflect')
    #dx = convnd(arr[t], xx, mode='reflect')
    #dy = convnd(arr[t], yy, mode='reflect')
    for h in range(arr.shape[1]):
      for w in range(arr.shape[2]):
        lines[t, h, w] = squaredSum(dx[0,h,w], dy[0,h,w], dz[0,h,w])
        #lines[t, h, w] = squaredSum(dx[h,w], dy[h,w])
  return lines 

def fakeGrayToGray(vidarr):
  # Had to copy the value three times to save it easily...
  return vidarr[:,:,:,0]

# from copyvid import *
# a = vidread('ahhh0.mp4')
# b = np.swapaxes(a, 0, 1)
# vidsave('ahhh_swap.mp4', b)

def test():
  crazyCool('swords_lines3.mp4', 'swords_gray.mp4')

def crazyCool(dstfile, srcfile):
  """dstfile is the lines video
  srcfile is a faked gray scale video"""
  print "Altering", srcfile , "to", dstfile
  arr = fakeGrayToGray(vidread(srcfile))
  res = inflate(timeAndSpace(arr))
  vidsave(dstfile, res)

def wow(dstfile, srcfile):
  """dstfile is the lines video
  srcfile is a raw video"""
  print "Altering", srcfile , "to", dstfile
  arr = fakeGrayToGray(grayFilter(vidread(srcfile)))
  res = inflate(timeAndSpace(arr))
  vidsave(dstfile, res)

if __name__ == '__main__':
  if len(sys.argv) > 0:
    wow(sys.argv[1], sys.argv[2])
  else:
    test()



