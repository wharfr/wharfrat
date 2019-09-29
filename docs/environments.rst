************
Environments
************

.. contents:: Table of Contents

Purpose
=======

Environments are a convienient way to access a set of executables from the host
system as if they are on the host.

They are based on approach taken by tools such as virtualenv, where the path is
modified to inject executables from the environment earlier into the search
path.

Creating an Environment
=======================

There are two parts to creating an environment:

 * Setting up one or more crates to export binaries
 * Creating the actual environment files

The first part is typically done once as part of the project configuration, and
checked in. The second is done by each person that wishes to use an environment
with that project.

Exporting Binaries
------------------

To export binaries from a crate into an environment the ``export-bin`` crate
setting. This setting consists of a list of patterns, and any executable that
matches any of these patterns will be exported into the environment.

Creating the Environment Files
------------------------------

The files for the environment are created using the ``wharfrat env create``
command. This command takes the path at which to create the environment. You can
also optionally select a subset of the crates in the project that should be used
to create the environment (the default being all crates in the project).

Using an Environment
====================

Once an environment has been created, then is must be activated to be used.
Activating the environment will change the shell's ``PATH`` such that the
exported binaries are first in the search path.

In addition to the binaries exported from the crates, the environment includes a
cached copy of the wharfrat binary that was used to create the environment.

Once an environment has been activated, any new exported binaries will be noted
against the commands that caused them to appear. This allows the commands to be
re-run when the create has to be recreated (which is done automatically when
running an exported command by default).

.. code-block:: bash

  ~/proj# wharfrat env create wrenv
  ~/proj# . wrenv/bin/activate
  (wr: wrenv) ~/proj#
