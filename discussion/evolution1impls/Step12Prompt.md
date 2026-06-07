# Prompt

Now that we have condocs in a good working state we will add the functionality to resume a condoc which was begun but not completed.

If the user attempts to enter a condoc filename that already exists when entering that mode then the handler will check if that file is a valid condoc.

If the file is a condoc the handler will then check if it is ongoing -- if it is completed it will complain and abort.

If the condoc is valid and has not been completed then the handler will begin where the condoc was left off.
