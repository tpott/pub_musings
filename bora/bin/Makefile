
default: encrypt decrypt

encrypt: encrypt.c
	# $@ is the left hand target
	# $^ is the right hand target
	$(CC) -o $@ $^

decrypt: decrypt.c
	$(CC) -o $@ $^

clean:
	rm encrypt decrypt
