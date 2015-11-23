# Python testing utility classes and functions
import os
import time
import traceback

def info(str):
    print "###################### " + str + " ######################"

def log(str):
    print "######## " + str

def exit(str):
    info("Test failed: " + str)
    traceback.print_stack()
    os._exit(1)
