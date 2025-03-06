FROM python:3.13-slim

# Install common Python libraries for data science and math
# Install as root, then we'll switch to non-root user
RUN pip install --no-cache-dir \
    numpy \
    pandas \
    matplotlib \
    scipy \
    sympy \
    scikit-learn \
    pillow

# Create a non-root user
RUN useradd -m nonrootuser

# Create and set permissions for /tmp directory
RUN mkdir -p /tmp && chmod 777 /tmp

# Set user to non-root
USER nonrootuser

# Set working directory
WORKDIR /tmp
