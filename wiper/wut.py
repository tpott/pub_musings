# wut.py
# Trevor Pottinger
# Mon Sep 19 15:40:18 PDT 2022

# `mount`
# /dev/disk3s1 on /Volumes/NO NAME (msdos, local, nodev, nosuid, noowners)

# `ls /Volumes/NO\ NAME`
# Each directory seems to be a separate backup, with some date, and likely nested structure

# `du -sh /Volumes/NO\ NAME/*`
# Shows which backups were the largest

# `sudo head -c 512 /dev/disk3s1 | hexdump -C`
# Requires to be run as root on mac, and requires the drive to be unmounted

# `sudo fuser /Volumes/NO\ NAME`
# Showed something...

# `sudo lsof /Volumes/NO\ NAME`
# Shows which files were open and who owns them

# `diskutil info /dev/disk2`
# Mac specific command, checks device block size and other info


def main() -> None:
	# for _ in range(n): read_bytes_random_offset_estimate_entropy()
	# for i in range(block_count): write_at(i, b"ff")
	# for _ in range(n): read_bytes_random_offset_estimate_entropy()
	# for i in range(block_count): write_rand_at(i)
	# for _ in range(n): read_bytes_random_offset_estimate_entropy()
	# for i in range(block_count): write_at(i, b"00")
	# for _ in range(n): read_bytes_random_offset_estimate_entropy()
	pass


if __name__ == '__main__':
	main()
