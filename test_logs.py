# Connectes to tcp connection and writes logs
import socket
import time
import random
import json

# Create a socket object
s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)

# Get local machine name
host = socket.gethostname()
port = 2253

s.connect((host, port))
# Send a thank you message to the client
s.send('<165>1 2023-10-15T14:30:45Z web01 nginx 1234 access [exampleSDID@32473 iut="3" eventSource="Application" eventID="1011"] User login successful\n'.encode('utf-8'))
# Close the connection
s.close()
