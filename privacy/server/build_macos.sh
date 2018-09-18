echo "install Lib"


# install boost
echo "install boost ... "
echo "brew install boost"
brew install boost

# install lib sodium
echo "install libsodium ... "
echo "brew install libsodium"
brew install libsodium

# install gmp
echo "install gmp ... "
echo "brew install gmp"
brew install gmp

# install lib protobuf-c
echo "install protobuf-c ... "
echo "brew install protobuf-c version 3.6.1"
brew install protobuf-c

# install grpc
echo "install grpc ... "
echo "brew install grpc"
brew install grpc

$BUILD_DIR = "build"
if [ -d "$BUILD_DIR" ]; then
  mkdir build
fi

cd build

cmake ..
make

file="./proving.key"
if [ -f "$file" ]
then
	echo "$file found."
else
	wget https://github.com/ninjadotorg/cash-prototype/releases/download/zkpp-v0.0.1/proving.key
fi

file="./verifying.key"
if [ -f "$file" ]
then
	echo "$file found."
else
	wget https://github.com/ninjadotorg/cash-prototype/releases/download/zkpp-v0.0.1/verifying.key
fi

./main
