# togray.py
# Trevor Pottinger
# Thu Mar 13 23:22:41 PDT 2014

import sys

import numpy as np
from scipy.signal import convolve2d as conv2d

# hopefully adds some nice display
from tqdm import tqdm

# lets try reading in a file and then writing it out
from moviepy.video.io.ffmpeg_reader import FFMPEG_VideoReader as reader
from moviepy.video.io.ffmpeg_writer import FFMPEG_VideoWriter as writer

def gcd(n, m):
  return 10 # lol

def resize(dstfile, srcfile, ratio=None):
  """Writes srcfile scaled by ratio (float) to dstfile. Resizes each 
  dimension by the ratio, so the area of each frame is actually 
  resized by ratio^2.
  
  If ratio is None, use 1.0 / gcd(width, height). 1.0 / ratio should 
  always be very close to an integer."""
  # shape = nframes, height, width, ?colordepth
  vidarr = vidread(srcfile)
  nframes, height, width = vidarr.shape[:3]
  if ratio == None:
    ratio = 1.0 / gcd(width, height)
  rate = int(1.0 / ratio + 0.5) # offset is so we round to nearest
  print 'resizing', srcfile, '-->', dstfile, 'by 10'
  print "vidarr.shape=", vidarr.shape
  print "rate", rate, "ratio", ratio
  smallervid = np.zeros([nframes, ratio * height, ratio * width, 3])
  print "smallervid.shape", smallervid.shape
  # rate -> X - rate + 1, wat, -> (1.0 - ratio) * X + 1, wat wat
  # NOTE: do not run
  #averagewindow = np.ones([(1.0 - ratio) * height + 1, (1.0 - ratio) * width + 1]) * (ratio ** 2)
  averagewindow = np.ones([rate, rate]) * (ratio ** 2)
  print "averagewindow.shape", averagewindow.shape
  for t in tqdm(range(nframes)):
    frame = vidarr[t]
    smallerframe = smallervid[t]
    for i in range(smallerframe.shape[0]):
      for j in range(smallerframe.shape[1]):
        window = frame[i*rate:(i+1)*rate,j*rate:(j+1)*rate]
        # set the pixel [i,j] rgb values
        smallerframe[i,j,0] = conv2d(window[:,:,0], averagewindow, mode='valid')
        smallerframe[i,j,1] = conv2d(window[:,:,1], averagewindow, mode='valid')
        smallerframe[i,j,2] = conv2d(window[:,:,2], averagewindow, mode='valid')
  return vidsave(dstfile, smallervid)

def rgb2gray(im):
  """Input is an rgb image and returns a grayscale image"""
  r, g, b = im[:,:,0], im[:,:,1], im[:,:,2]
  return 0.2989 * r + 0.5870 * g + 0.1140 * b

def add_dimension(im, length=3):
  """Duplicates the values in im across all the values in range(length)"""
  bigger = np.zeros(list(im.shape) + [length])
  for h in range(im.shape[0]):
    for w in range(im.shape[1]):
      bigger[h, w, 0] = im[h, w]
      bigger[h, w, 1] = im[h, w]
      bigger[h, w, 2] = im[h, w]
  return bigger

def gray(dstfile, srcfile):
  """Reads in srcfile in a rgb video and saves grayscale video to
  dstfile"""
  print "Converting", srcfile, "to grayscale ->", dstfile
  vidarr = vidread(srcfile)
  return vidsave(dstfile, grayFilter(vidarr))

def grayFilter(vidarr):
  grayarr = np.zeros([vidarr.shape[0], vidarr.shape[1], vidarr.shape[2], 3])
  for t in tqdm(range(vidarr.shape[0]), desc="Grayscale"):
    np.copyto(grayarr[t], add_dimension(rgb2gray(vidarr[t])))
  return grayarr

def vidcopy(dstfile, srcfile):
  vidarr = vidread(srcfile)
  return vidsave(dstfile, vidarr)

def vidread(filename):
  f = reader(filename)
  vidarr = np.zeros([f.nframes, f.size[1], f.size[0], 3])
  #vidarr = np.zeros([10, f.size[1], f.size[0], 3])
  np.copyto(vidarr[0], f.lastread)
  for i in range(1, f.nframes):
    np.copyto(vidarr[i], f.read_frame())
  return vidarr

def vidsave(filename, vidarr):
  # TODO save the fps? 
  fps = 30.
  w = writer(filename, (vidarr.shape[2], vidarr.shape[1]), fps)
  for t in range(vidarr.shape[0]):
    # TODO don't convert when type is already correct
    w.write_frame(vidarr[t].astype('uint8'))
  return w.close()

# from copyvid import *
# a = vidread('ahhh0.mp4')
# b = np.swapaxes(a, 0, 1)
# vidsave('ahhh_swap.mp4', b)

if __name__ == '__main__':
  if len(sys.argv) > 0:
    gray(sys.argv[1], sys.argv[2])
  else:
    gray('swords_gray.mp4', 'swords.mp4')
