# hcreport <img src="img/hcreport.png" alt="hcreport" style="height: 26px"/>
## A collection of Openshift and k8s health checks bundled in a operator 

Our CSA LATAM team has been facing the challenge of reviewing Openshift clusters, managed or not, during too many of our engagements and the current set of scripts to collect and transform data into a nice report not sparsely faces execution problems by the customers so I decided to repack them into a operator.

This would come as a personal exercise for understanding and using operators as much as getting to know [Go](https://go.dev/) better. 

The general idea is to replicate all the action used to collect cluster data aggregating the result in its raw yaml form comprising of various files as a first stage result, transform it using pre-configured [jq](https://jqlang.github.io/jq/) and [yq](https://mikefarah.gitbook.io/yq/) queries as needed into a secondary stage result also in the form of yaml files now with a customized group of objects, lists and such which will be used as input data to a static site generator based on [Markdown](https://daringfireball.net/projects/markdown/) files. 

At stage 3 the static generator of your choice ([Jekyll](https://jekyllrb.com/)?, [MkDocs+Materials](https://www.mkdocs.org/)?) will use its templating language ([Liquid](https://shopify.github.io/liquid/), [Jinja](https://jinja.palletsprojects.com/en/3.1.x/)) to manipulate the yaml gathered data from stage 2 as context for a static documentation site. A natural stage 4 can offer from the generated site, or automatically, the documentation converted into the pdf format ([Markdown to file](https://github.com/wll8/markdown-to-file#markdown-pdfoutputDirectory), [MkPDFs for MkDocs](https://comwes.github.io/mkpdfs-mkdocs-plugin/index.html)) as a collection of reports.

The goal is to use Go all around and all the way so much kubectl/oc, yq and jq are going to be used as libraries for running pre-fabricated commands outside the code as configuration items probably sourced from a yaml config file. This will also bring extra freedom and power for those not familiar with go to later maintain the hcreports. Let's look at it as a toolbox operator driven by external configuration capable of extracting cluster data in a regular loop for generating nice reports. The idea is producing beautiful pdf reports for client's appreciation and guidance regarding best practices.

The resulting static site and pdf files storage is yet undefined, but it is our will to use a git repository to continuously keep them versioned for each reconcile loop executed that generates reports. This would be a efficient and cost effective means for tracking cluster changes in time.
