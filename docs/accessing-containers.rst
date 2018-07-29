********************
Accessing Containers
********************

.. contents:: Table of Contents

wharfrat run
============

Exposing Commands
=================

It is possible to use the ``wr-exec`` command to expose commands from inside a
container to the host. This is normally done by creating an executable config
file with a ``#!`` line that invokes ``wr-exec``. For example, if ``./test``
contains:

.. code-block:: toml

  #!/usr/bin/env wr-exec

  project = "/path/to/project/file"
  command = ["command", "arg1"]

Then, running ``./test arg2`` will run the command ``command arg1 arg2`` in the
container for the default crate defined in the wharfrat project file at
``/path/to/project/file``.
