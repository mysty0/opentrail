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
s.send('2023-12-01T10:30:00Z|INFO|user123|Application started successfully\n'.encode('utf-8'))
# Close the connection
s.close()
