# xdr

The xdr package contains encoding/decoding of Stellar XDR types.

## Code Generate

Most of the code this package is generated.

To download new XDR for code generation:

```
docker run --platform linux/amd64 -it --rm -v $PWD:/wd -w /wd ruby /bin/bash -c 'bundle install && bundle exec rake xdr:download'
```

To regenerate the code from the local XDR:

```
docker run --platform linux/amd64 -it --rm -v $PWD:/wd -w /wd ruby /bin/bash -c 'bundle install && bundle exec rake xdr:generate' && go fmt ./xdr
```

To download XDR for a different branch of stellar-core, modify `Rakefile` in the root.

## Zero-copy views and the Walk

The generated `XView` types read XDR wire bytes without decoding, and the
generated `WalkX` visitors extract from them in a single pass. Doctrine: read
a little → views; consume a lot → Walk. See
[`xdrgen/views_api.md`](../xdrgen/views_api.md) for the full contract
(accessors, iterators, `Must*`/`Try*`, Walk positions and upgrade policy).
