# vidcopy.py
# Trevor Pottinger
# Thu Mar  6 17:18:22 PST 2014

import numpy as np

# lets try reading in a file and then writing it out
from moviepy.video.io.ffmpeg_reader import FFMPEG_VideoReader as reader
from moviepy.video.io.ffmpeg_writer import FFMPEG_VideoWriter as writer

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
