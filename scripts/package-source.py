"""Package committed corresponding source plus pinned Go dependency sources."""
import argparse
from pathlib import Path
import subprocess
import tempfile
import zipfile

root=Path(__file__).resolve().parent.parent
parser=argparse.ArgumentParser(description=__doc__)
parser.add_argument('--version',required=True)
parser.add_argument('--output',type=Path,required=True)
args=parser.parse_args()
args.output.mkdir(parents=True,exist_ok=True)
subprocess.run(['git','diff','--exit-code','HEAD'],cwd=root,check=True,stdout=subprocess.DEVNULL)
with tempfile.TemporaryDirectory(prefix='usb-relay-source-') as temporary:
    temporary=Path(temporary)
    archive=temporary/'committed.zip'
    subprocess.run(['git','archive','--format=zip','--output',str(archive),'HEAD'],cwd=root,check=True)
    stage=temporary/'source';stage.mkdir()
    with zipfile.ZipFile(archive) as source:
        for entry in source.infolist():
            path=(stage/entry.filename).resolve()
            if not path.is_relative_to(stage.resolve()):raise RuntimeError('Unsafe source path')
        source.extractall(stage)
    subprocess.run(['go','mod','vendor','-o',str(stage/'vendor')],cwd=root,check=True)
    output=args.output/('USB-Relay-'+args.version+'-source.zip')
    with zipfile.ZipFile(output,'w',zipfile.ZIP_DEFLATED,compresslevel=6) as result:
        for path in sorted(stage.rglob('*')):
            if path.is_file():result.write(path,path.relative_to(stage))
    print(output.name)
